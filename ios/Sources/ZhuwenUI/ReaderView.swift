import SwiftUI
import ZhuwenCore
import ZhuwenPacks

/// The story reader (handoff §5, FR-4): word-segmented text with tap-to-gloss. Frontier
/// words carry a cinnabar dotted underline (FR-4.2). Songti serif for story text (FR-4.5).
public struct ReaderView: View {
    private let model: ReaderModel
    private let listen: (() -> ListeningModel)?
    private let comprehension: (() -> ComprehensionView)?
    private let tapWord: ((Int) -> Void)?
    @State private var selectedGloss: Gloss?
    @State private var showListening = false
    @State private var showComprehension = false

    public init(model: ReaderModel, listen: (() -> ListeningModel)? = nil,
                comprehension: (() -> ComprehensionView)? = nil,
                tapWord: ((Int) -> Void)? = nil) {
        self.model = model
        self.listen = listen
        self.comprehension = comprehension
        self.tapWord = tapWord
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
                tapWord?(wordID)
            } label: {
                text.underline(token.isFrontier, pattern: .dot, color: .cinnabar)
            }
            .buttonStyle(.plain)
            .accessibilityIdentifier("readerToken")
        } else {
            text // literals / proper nouns are not tappable in CP-03
        }
    }
}

/// The gloss sheet shown on word tap (FR-4.1: form, tone pinyin, English meaning, HSK level).
struct GlossSheet: View {
    let gloss: Gloss

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(gloss.simp).font(.custom("Songti SC", size: 40)).foregroundColor(Palette.ink)
            Text(gloss.pinyin).font(.title3).foregroundColor(Palette.cinnabar)
            if !gloss.en.isEmpty {
                Text(gloss.en).font(.title3).foregroundColor(Palette.ink2)
            }
            Divider().background(Palette.hairline)
            HStack {
                Label("HSK \(gloss.hsk3Level)", systemImage: "graduationcap")
                Spacer()
                Label("rank \(gloss.freqRank)", systemImage: "number")
            }
            .font(.footnote)
            .foregroundColor(Palette.ink3)
            Spacer()
        }
        .padding()
        .adaptiveGlass(cornerRadius: 20)
        #if os(iOS)
        .presentationDetents([.height(240)])
        #endif
    }
}
