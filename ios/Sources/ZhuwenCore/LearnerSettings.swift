import Foundation

/// LearnerSettings holds the on-device preferences (00 §4 FR-10.1) plus the opt-in iCloud sync flag
/// (FR-10.2, **off by default**). It is a plain Codable value the app persists (UserDefaults/SwiftData)
/// and the `SettingsView` binds to. No preference is a secret and none leaves the device except, if the
/// learner explicitly turns it on, learner state to their *private* CloudKit DB (I2).
public struct LearnerSettings: Codable, Equatable, Sendable {
    /// How pinyin is shown in the reader (FR-4).
    public enum PinyinMode: String, Codable, CaseIterable, Sendable {
        case always, frontierOnly, onTap, never
    }
    /// Reader theme (FR-10.1).
    public enum Theme: String, Codable, CaseIterable, Sendable {
        case system, light, sepia, dark
    }
    /// Which narration voice to use (FR-5.4 / FR-10.1).
    public enum AudioVoice: String, Codable, CaseIterable, Sendable {
        case pack        // the pack's rendered narration when present
        case systemTTS   // always use system TTS (labeled)
    }

    public var pinyinMode: PinyinMode
    public var frontierUnderline: Bool
    public var theme: Theme
    /// Reader font size in points (Dynamic Type still applies; NFR-6).
    public var readerFontSize: Double
    public var audioVoice: AudioVoice
    /// Learner-set daily review cap; clamped against the tier cap by `FeatureGate` (FR-7.2/9.1).
    public var dailyReviewCap: Int
    /// FR-10.2: optional private-CloudKit sync, **off by default**.
    public var iCloudSyncEnabled: Bool

    public init(pinyinMode: PinyinMode = .frontierOnly,
                frontierUnderline: Bool = true,
                theme: Theme = .system,
                readerFontSize: Double = 20,
                audioVoice: AudioVoice = .pack,
                dailyReviewCap: Int = 20,
                iCloudSyncEnabled: Bool = false) {
        self.pinyinMode = pinyinMode
        self.frontierUnderline = frontierUnderline
        self.theme = theme
        self.readerFontSize = readerFontSize
        self.audioVoice = audioVoice
        self.dailyReviewCap = dailyReviewCap
        self.iCloudSyncEnabled = iCloudSyncEnabled
    }

    public static let `default` = LearnerSettings()

    public func encoded() throws -> Data { try JSONEncoder().encode(self) }

    public static func decoded(from data: Data) -> LearnerSettings {
        (try? JSONDecoder().decode(LearnerSettings.self, from: data)) ?? .default
    }
}
