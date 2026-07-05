import SwiftUI
import ZhuwenCore
import ZhuwenPacks

/// Drives the placement flow (M1–M3) for SwiftUI. All decisions live in `ZhuwenCore`
/// (`PlacementSession` / `PlacementEstimator`); this only republishes and runs the estimate
/// when the word check finishes. The produced `PlacementResult.seed` feeds `KnownWordModel`.
@MainActor
public final class PlacementFlowModel: ObservableObject {
    @Published public private(set) var session: PlacementSession
    @Published public private(set) var result: PlacementResult?

    private let lexicon: LexiconStore
    private let estimator: PlacementEstimator
    public var onComplete: ((PlacementResult) -> Void)?

    public init(lexicon: [WordRecord], itemCount: Int = 90, seed: UInt64 = 0xF00D,
                estimator: PlacementEstimator = PlacementEstimator()) {
        let store = LexiconStore(lexicon)
        self.lexicon = store
        self.estimator = estimator
        let items = PlacementItemBuilder(itemCount: itemCount, seed: seed).build(lexicon: store)
        self.session = PlacementSession(items: items)
    }

    public var phase: PlacementSession.Phase { session.phase }
    public var currentItem: PlacementItem? { session.currentItem }
    public var stepLabel: String { "\(min(session.answeredCount + 1, session.total)) of \(session.total)" }
    public var progress: Double {
        session.total == 0 ? 0 : Double(session.answeredCount) / Double(session.total)
    }
    public var maxRank: Int { lexicon.words.map { $0.freqRank }.max() ?? 6000 }

    public func begin() { session.begin() }

    public func skipAsBeginner() {
        session.skipAsBeginner()
        finish()
    }

    public func answer(known: Bool) {
        session.answer(known: known)
        if session.isComplete { finish() }
    }

    private func finish() {
        let r = session.beginner
            ? PlacementResult.beginner
            : estimator.estimate(items: session.items, answers: session.answers, lexicon: lexicon)
        result = r
        onComplete?(r)
    }
}

/// The placement screens (M1 welcome · M2 word check · M3 result). Presented on first run or
/// re-run from Settings (FR-1.5).
public struct PlacementView: View {
    @StateObject private var model: PlacementFlowModel
    private let onFinish: (PlacementResult?) -> Void

    public init(model: @autoclosure @escaping () -> PlacementFlowModel,
                onFinish: @escaping (PlacementResult?) -> Void = { _ in }) {
        _model = StateObject(wrappedValue: model())
        self.onFinish = onFinish
    }

    public var body: some View {
        switch model.phase {
        case .welcome: WelcomeScreen(model: model)          // M1
        case .wordCheck: WordCheckScreen(model: model)      // M2
        case .result:                                       // M3
            if let r = model.result {
                ResultScreen(result: r, maxRank: model.maxRank) { onFinish(r) }
            } else {
                Color.clear.onAppear { onFinish(nil) }
            }
        }
    }
}

// MARK: - M1 Welcome / privacy

private struct WelcomeScreen: View {
    @ObservedObject var model: PlacementFlowModel

    var body: some View {
        VStack(spacing: 0) {
            Spacer()
            RoundedRectangle(cornerRadius: 20)
                .fill(Color.cinnabar)
                .frame(width: 112, height: 112)
                .overlay(Text("朱").font(.system(size: 56, weight: .semibold)).foregroundColor(.white))
            Text("Zhuwen").font(.system(size: 34, weight: .semibold)).padding(.top, 28)
            Text("Read Mandarin that is 98% words you already know. The other 2% is how you grow.")
                .multilineTextAlignment(.center).foregroundColor(.secondary)
                .padding(.horizontal, 32).padding(.top, 8)
            VStack(alignment: .leading, spacing: 10) {
                Label("No account — there is nothing to sign into", systemImage: "checkmark")
                Label("No tracking, no ads, ever", systemImage: "checkmark")
                Label("Everything stays on your iPhone", systemImage: "checkmark")
            }
            .font(.subheadline).foregroundColor(.secondary).padding(.top, 28)
            Spacer()
            VStack(spacing: 10) {
                Button { model.begin() } label: {
                    Text("Take the 4-minute placement").frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent).tint(.cinnabar)
                Button("I'm a complete beginner") { model.skipAsBeginner() }
                    .foregroundColor(.secondary)
            }
            .padding(.horizontal, 20).padding(.bottom, 24)
        }
    }
}

// MARK: - M2 Word check

private struct WordCheckScreen: View {
    @ObservedObject var model: PlacementFlowModel

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Button("Cancel") { model.skipAsBeginner() }.foregroundColor(.secondary)
                Spacer()
                Text(model.stepLabel).font(.subheadline.weight(.semibold)).foregroundColor(.secondary)
                Spacer()
                Color.clear.frame(width: 44)
            }
            .padding(.horizontal, 16).padding(.top, 8)
            ProgressView(value: model.progress).tint(.cinnabar).padding(.horizontal, 8).padding(.top, 4)

            Spacer()
            Text("Do you know this word?").font(.footnote.weight(.semibold))
                .foregroundColor(.secondary).textCase(.uppercase)
            Text(model.currentItem?.surface ?? "")
                .font(.system(size: 76, weight: .bold)).padding(.top, 20)
            Text("Be honest — pinyin and meaning stay hidden.\nGuessing inflates your level and makes stories harder.")
                .font(.footnote).multilineTextAlignment(.center)
                .foregroundColor(.secondary).padding(.top, 16).padding(.horizontal, 24)
            Spacer()

            HStack(spacing: 12) {
                Button { model.answer(known: false) } label: {
                    Text("Not yet").frame(maxWidth: .infinity)
                }
                .buttonStyle(.bordered)
                Button { model.answer(known: true) } label: {
                    Text("I know it").frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent).tint(.jade)
            }
            .padding(.horizontal, 20).padding(.bottom, 24)
        }
    }
}

// MARK: - M3 Result

private struct ResultScreen: View {
    let result: PlacementResult
    let maxRank: Int
    let onContinue: () -> Void

    var body: some View {
        VStack(spacing: 16) {
            Text("Your starting point").font(.footnote.weight(.semibold))
                .foregroundColor(.secondary).textCase(.uppercase)
                .frame(maxWidth: .infinity, alignment: .leading)

            HStack(spacing: 12) {
                StatCard(value: result.cefr.label, caption: "CEFR reading\n(estimate)", tint: .cinnabar)
                StatCard(value: result.hskLevel == 0 ? "—" : "\(result.hskLevel)",
                         caption: "HSK 3.0 level\n(estimate)", tint: .primary)
            }

            VStack(alignment: .leading, spacing: 8) {
                Text("≈ \(result.estimatedKnownCount) words likely known").font(.headline)
                Text("Probability of knowing a word, by frequency rank")
                    .font(.caption).foregroundColor(.secondary)
                CurveBars(samples: result.curveSamples(maxRank: maxRank))
                    .frame(height: 96).padding(.top, 6)
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(16)
            .background(RoundedRectangle(cornerRadius: 14).fill(Color.secondary.opacity(0.08)))

            if result.route == .foundations {
                Text("We'll start you in Foundations to build a base before stories.")
                    .font(.footnote).foregroundColor(.secondary).multilineTextAlignment(.center)
            }
            Spacer()
            Button(action: onContinue) {
                Text(result.route == .foundations ? "Start Foundations" : "Start reading")
                    .frame(maxWidth: .infinity)
            }
            .buttonStyle(.borderedProminent).tint(.cinnabar)
        }
        .padding(20)
    }
}

private struct StatCard: View {
    let value: String
    let caption: String
    let tint: Color

    var body: some View {
        VStack(spacing: 4) {
            Text(value).font(.system(size: 44, weight: .heavy)).foregroundColor(tint)
            Text(caption).font(.caption2).foregroundColor(.secondary).multilineTextAlignment(.center)
        }
        .frame(maxWidth: .infinity).padding(16)
        .background(RoundedRectangle(cornerRadius: 14).fill(Color.secondary.opacity(0.08)))
    }
}

private struct CurveBars: View {
    let samples: [Double]

    var body: some View {
        GeometryReader { geo in
            HStack(alignment: .bottom, spacing: 3) {
                ForEach(Array(samples.enumerated()), id: \.offset) { _, p in
                    RoundedRectangle(cornerRadius: 3)
                        .fill(Color.jade.opacity(0.35 + 0.65 * p))
                        .frame(height: max(2, CGFloat(p) * geo.size.height))
                        .frame(maxWidth: .infinity)
                }
            }
        }
    }
}
