import SwiftUI
import ZhuwenCore
import ZhuwenPacks

/// The story reader (handoff §5, FR-4): word-segmented text with tap-to-gloss. Frontier
/// words carry a cinnabar dotted underline (FR-4.2). Songti serif for story text (FR-4.5).
public struct ReaderView: View {
    private let model: ReaderModel
    private let listen: (() -> ListeningModel)?
    private let comprehension: (() -> ComprehensionView)?
    @State private var selectedGloss: Gloss?
    @State private var showListening = false
    @State private var showComprehension = false

    public init(model: ReaderModel, listen: (() -> ListeningModel)? = nil,
                comprehension: (() -> ComprehensionView)? = nil) {
        self.model = model
        self.listen = listen
        self.comprehension = comprehension
    }

    public var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                Text(model.story.titleZH)
                    .font(.title2).bold()
                FlowLayout(spacing: 0, lineSpacing: 10) {
                    ForEach(model.tokens()) { token in
                        tokenView(token)
                    }
                }
                if comprehension != nil {
                    Button {
                        showComprehension = true
                    } label: {
                        Text("Finish — check comprehension").frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.borderedProminent).tint(.cinnabar)
                    .padding(.top, 8)
                }
            }
            .padding()
        }
        .toolbar {
            if listen != nil {
                Button {
                    showListening = true
                } label: {
                    Image(systemName: "headphones")
                }
                .accessibilityLabel("Listen")
            }
        }
        .navigationDestination(isPresented: $showListening) {
            if let listen {
                ListeningView(title: model.story.titleZH, model: listen())
            }
        }
        .sheet(isPresented: $showComprehension) {
            if let comprehension {
                comprehension()
            }
        }
        .sheet(item: $selectedGloss) { gloss in
            GlossSheet(gloss: gloss)
        }
    }

    @ViewBuilder
    private func tokenView(_ token: DisplayToken) -> some View {
        let text = Text(token.text)
            .font(.custom("Songti SC", size: 22))
            .foregroundColor(token.isFrontier ? .cinnabar : .primary)

        if let wordID = token.wordID {
            Button {
                selectedGloss = model.gloss(for: wordID)
            } label: {
                text.underline(token.isFrontier, pattern: .dot, color: .cinnabar)
            }
            .buttonStyle(.plain)
        } else {
            text // literals / proper nouns are not tappable in CP-03
        }
    }
}

/// The gloss sheet shown on word tap (FR-4.1 subset: form, tone pinyin, HSK level).
struct GlossSheet: View {
    let gloss: Gloss

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(gloss.simp).font(.custom("Songti SC", size: 40))
            Text(gloss.pinyin).font(.title3).foregroundColor(.cinnabar)
            Divider()
            HStack {
                Label("HSK \(gloss.hsk3Level)", systemImage: "graduationcap")
                Spacer()
                Label("rank \(gloss.freqRank)", systemImage: "number")
            }
            .font(.footnote)
            .foregroundColor(.secondary)
            Spacer()
        }
        .padding()
        #if os(iOS)
        .presentationDetents([.height(220)])
        #endif
    }
}
