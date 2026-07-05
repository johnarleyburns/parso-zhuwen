import Foundation

/// A minimal read-only ZIP reader for STORED (uncompressed) entries, driven by the
/// central directory (authoritative sizes/offsets — robust against streaming data
/// descriptors). Dependency-free and iOS-safe. Zhuwen packs are written STORED
/// (see PACK_FORMAT.md), so no inflate is required.
public struct ZipArchive {
    public enum ZipError: Error, Equatable {
        case notAZip
        case truncated
        case unsupportedCompression(method: UInt16, entry: String)
        case badEntry(String)
    }

    private let data: Data
    /// Entry name -> (dataOffset, size) for STORED entries.
    private var entries: [String: (offset: Int, size: Int)] = [:]
    public private(set) var names: [String] = []

    public init(data: Data) throws {
        self.data = data
        try parseCentralDirectory()
    }

    public init(url: URL) throws {
        try self.init(data: Data(contentsOf: url))
    }

    public func contains(_ name: String) -> Bool { entries[name] != nil }

    /// Returns the raw bytes of a STORED entry, or nil if absent.
    public func data(for name: String) -> Data? {
        guard let e = entries[name] else { return nil }
        return data.subdata(in: e.offset ..< (e.offset + e.size))
    }

    // MARK: - Parsing

    private static let eocdSig: UInt32 = 0x0605_4b50
    private static let centralSig: UInt32 = 0x0201_4b50
    private static let localSig: UInt32 = 0x0403_4b50

    private mutating func parseCentralDirectory() throws {
        let n = data.count
        guard n >= 22 else { throw ZipError.truncated }

        // Locate End Of Central Directory record by scanning backward.
        var eocd = -1
        let minStart = max(0, n - (22 + 0xFFFF))
        var i = n - 22
        while i >= minStart {
            if readU32(i) == Self.eocdSig { eocd = i; break }
            i -= 1
        }
        guard eocd >= 0 else { throw ZipError.notAZip }

        let count = Int(readU16(eocd + 10))
        var p = Int(readU32(eocd + 16)) // offset of central directory

        for _ in 0 ..< count {
            guard p + 46 <= n, readU32(p) == Self.centralSig else { throw ZipError.truncated }
            let method = readU16(p + 10)
            let compSize = Int(readU32(p + 20))
            let nameLen = Int(readU16(p + 28))
            let extraLen = Int(readU16(p + 30))
            let commentLen = Int(readU16(p + 32))
            let localOff = Int(readU32(p + 42))
            guard p + 46 + nameLen <= n else { throw ZipError.truncated }
            let name = String(decoding: data.subdata(in: (p + 46) ..< (p + 46 + nameLen)), as: UTF8.self)

            // Resolve data start via the local file header (name/extra lengths may differ).
            guard localOff + 30 <= n, readU32(localOff) == Self.localSig else {
                throw ZipError.badEntry(name)
            }
            if method != 0 { throw ZipError.unsupportedCompression(method: method, entry: name) }
            let lNameLen = Int(readU16(localOff + 26))
            let lExtraLen = Int(readU16(localOff + 28))
            let dataStart = localOff + 30 + lNameLen + lExtraLen
            guard dataStart + compSize <= n else { throw ZipError.truncated }

            entries[name] = (offset: dataStart, size: compSize)
            names.append(name)
            p += 46 + nameLen + extraLen + commentLen
        }
    }

    private func readU16(_ off: Int) -> UInt16 {
        UInt16(data[data.startIndex + off]) | (UInt16(data[data.startIndex + off + 1]) << 8)
    }

    private func readU32(_ off: Int) -> UInt32 {
        let b = data.startIndex + off
        return UInt32(data[b])
            | (UInt32(data[b + 1]) << 8)
            | (UInt32(data[b + 2]) << 16)
            | (UInt32(data[b + 3]) << 24)
    }
}
