import SwiftUI

enum Palette {
    // Background gradient — warm ink with a hint of cinnabar/paper warmth.
    static let bgStart = Color(hex: 0x0F0E0C)
    static let bgEnd = Color(hex: 0x080706)

    static var backgroundGradient: LinearGradient {
        LinearGradient(
            stops: [
                .init(color: Color(hex: 0x1A1512), location: 0),
                .init(color: Color(hex: 0x0F0E0C), location: 0.35),
                .init(color: Color(hex: 0x080706), location: 1)
            ],
            startPoint: .top, endPoint: .bottom
        )
    }

    // Ink hierarchy (zhuwen's paper/ink identity).
    static let ink = Color(hex: 0xF2F4F6)
    static let ink2 = Color(white: 0.92).opacity(0.58)
    static let ink3 = Color(white: 0.92).opacity(0.34)

    // Cinnabar (#C3272B) — the scholarly red seal accent.
    static let cinnabar = Color(hex: 0xC3272B)
    static let cinnabarDim = Color(hex: 0x9A1F23)

    // Jade (#2E7D5B) — completion / progress.
    static let jade = Color(hex: 0x2E7D5B)

    // Gold seal accent.
    static let gold = Color(hex: 0xD4A543)
    static let goldDim = Color(hex: 0xA67F2E)

    // Semantic.
    static let ok = Color(hex: 0x4CD471)
    static let danger = Color(hex: 0xFF6B5E)

    // Hairline stroke.
    static let hairline = Color.white.opacity(0.10)

    // Glass surface fill (toned to zhuwen's warm palette).
    static let glassFill = Color(hex: 0x1B1512).opacity(0.55)

    // Opaque fallback for Reduce Transparency.
    static let opaqueFallback = Color(hex: 0x1C1816)
}

extension Color {
    init(hex: UInt32) {
        let r = Double((hex >> 16) & 0xFF) / 255
        let g = Double((hex >> 8) & 0xFF) / 255
        let b = Double(hex & 0xFF) / 255
        self.init(.sRGB, red: r, green: g, blue: b, opacity: 1)
    }
}
