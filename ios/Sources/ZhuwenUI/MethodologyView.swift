import SwiftUI

/// The methodology / honest-limits page (I4, 00 §5A.4). Foundations makes an evidence-gated
/// claim about picture-word binding; this page states where the method is strong and where it
/// deliberately degrades, so the product never over-claims (FR-6.3 estimate labeling ethos).
public struct MethodologyView: View {
    @Environment(\.dismiss) private var dismiss

    public init() {}

    public var body: some View {
        NavigationStack {
            List {
                Section("How Foundations works") {
                    Text("Foundations builds your first ~300 words from real photographs. Each word is introduced with a picture, its sound, and its characters, then checked by recognition and reading — never as an isolated flashcard. Short sessions end with a little story that recombines what you just learned.")
                        .font(.footnote)
                }
                Section("Where picture-binding is strong") {
                    Text("Concrete, photographable words — animals, food, family, numbers, colors, everyday actions — bind cleanly to an image. These carry the bulk of the Foundations course.")
                        .font(.footnote)
                }
                Section("Honest limits") {
                    Text("Picture-word binding **degrades for abstraction**. Words like grammar particles or abstract ideas can't be photographed, so Foundations teaches them through pattern sentences instead of pictures, and defers the most abstract vocabulary to real stories.")
                        .font(.footnote)
                    Text("A one-line English gloss is available **behind a tap**, off by default. It is a fallback for a genuinely stuck moment, not the primary path — the method works by binding meaning to image and sound, not to translation.")
                        .font(.footnote)
                }
                Section("Images") {
                    Text("Every photograph is a real photograph or public-domain artwork from Wikimedia Commons, shipped with its author and license. No imagery is AI-generated, anywhere in the app.")
                        .font(.footnote)
                }
                Section {
                    Text("Your level is always shown as an estimate, and everything you do stays on your device.")
                        .font(.caption).foregroundColor(.secondary)
                }
            }
            .navigationTitle("Method & limits")
            .toolbar {
                ToolbarItem(placement: .confirmationAction) { Button("Done") { dismiss() } }
            }
        }
    }
}
