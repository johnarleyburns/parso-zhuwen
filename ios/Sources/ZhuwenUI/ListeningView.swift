import SwiftUI
import ZhuwenAudio
import ZhuwenCore
import ZhuwenPacks

/// The listening screen (M7, FR-5): karaoke word-highlight synced to pack audio, playback
/// speed 0.6×–1.2×, blind-listening toggle, and a "System voice" label when the on-device TTS
/// fallback is used (FR-5.4). Tap a word to seek (FR-5.1).
public struct ListeningView: View {
    @StateObject private var model: ListeningModel
    private let title: String

    public init(title: String, model: @autoclosure @escaping () -> ListeningModel) {
        self.title = title
        _model = StateObject(wrappedValue: model())
    }

    public var body: some View {
        VStack(spacing: 0) {
            storyText
            controls
        }
        .navigationTitle(title)
        .onDisappear { model.stop() }
    }

    private var storyText: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 12) {
                if model.usingSystemVoice {
                    Label("System voice — download the audio pack for the narrated version",
                          systemImage: "speaker.wave.2")
                        .font(.caption).foregroundColor(.secondary)
                        .padding(.bottom, 4)
                }
                if model.textHidden {
                    blindPlaceholder
                } else {
                    FlowLayout(spacing: 0, lineSpacing: 10) {
                        ForEach(model.tokens) { token in
                            tokenView(token)
                        }
                    }
                }
            }
            .padding()
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    @ViewBuilder
    private func tokenView(_ token: DisplayToken) -> some View {
        let isCurrent = model.highlightedIndex == token.id
        let text = Text(token.text)
            .font(.custom("Songti SC", size: 24))
            .foregroundColor(isCurrent ? .white : .primary)

        Button {
            model.seek(toToken: token.id)
        } label: {
            text
                .padding(.horizontal, 1)
                .background(
                    RoundedRectangle(cornerRadius: 4)
                        .fill(isCurrent ? Color.cinnabar : Color.clear)
                )
        }
        .buttonStyle(.plain)
        .animation(.easeOut(duration: 0.12), value: isCurrent)
    }

    private var blindPlaceholder: some View {
        VStack(spacing: 16) {
            Image(systemName: "ear").font(.system(size: 44)).foregroundColor(.jade)
            Text("Listen first").font(.headline)
            Text("Text is hidden. Listen to the story, then reveal to check yourself.")
                .font(.subheadline).foregroundColor(.secondary)
                .multilineTextAlignment(.center)
            Button("Reveal text") { model.reveal() }
                .buttonStyle(.bordered).tint(.jade)
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 40)
    }

    private var controls: some View {
        VStack(spacing: 14) {
            Divider()
            ProgressView(value: model.progress).tint(.cinnabar)

            HStack {
                Toggle(isOn: Binding(get: { model.blindMode }, set: { _ in model.toggleBlind() })) {
                    Label("Blind", systemImage: "eye.slash")
                }
                .toggleStyle(.button)
                .tint(.jade)

                Spacer()

                Button {
                    model.playPause()
                } label: {
                    Image(systemName: model.isPlaying ? "pause.circle.fill" : "play.circle.fill")
                        .font(.system(size: 52))
                        .foregroundColor(.cinnabar)
                }

                Spacer()

                Menu {
                    ForEach(PlaybackSpeed.presets, id: \.value) { s in
                        Button(s.label) { model.setSpeed(s) }
                    }
                } label: {
                    Text(model.speed.label)
                        .font(.subheadline.weight(.semibold))
                        .frame(width: 52)
                }
            }
            .padding(.horizontal, 8)
        }
        .padding(.horizontal, 16)
        .padding(.bottom, 12)
    }
}
