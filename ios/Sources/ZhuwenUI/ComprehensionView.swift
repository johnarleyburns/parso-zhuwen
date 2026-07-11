import SwiftUI
import ZhuwenCore
import ZhuwenPacks

/// The comprehension check + seal (M8, FR-6.1/6.2). Three MC questions in Chinese; passing stamps
/// the story with the 读完为证 seal (stamp animation, or a fade under Reduce Motion, NFR-6) and
/// boosts P(known) for every exposed word.
public struct ComprehensionView: View {
    @ObservedObject private var learner: LearnerModel
    private let story: StoryRecord
    private let onDone: () -> Void

    @State private var session: ComprehensionSession
    @State private var selected: Int?
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    public init(learner: LearnerModel, story: StoryRecord, onDone: @escaping () -> Void = {}) {
        self.learner = learner
        self.story = story
        self.onDone = onDone
        _session = State(initialValue: learner.comprehensionSession(for: story))
    }

    public var body: some View {
        VStack(spacing: 20) {
            if session.isComplete {
                resultScreen
            } else if let q = session.currentQuestion {
                questionScreen(q)
            } else {
                // No questions in this pack — nothing to check.
                Text("No comprehension questions for this story.")
                    .foregroundColor(.secondary)
                Button("Done", action: onDone).buttonStyle(.borderedProminent).tint(.cinnabar)
            }
        }
        .padding(20)
        .adaptiveGlass(cornerRadius: 20)
        .animation(reduceMotion ? nil : .spring(response: 0.4, dampingFraction: 0.6), value: session.isComplete)
    }

    // MARK: - Question (M8)

    private func questionScreen(_ q: QuestionRecord) -> some View {
        VStack(alignment: .leading, spacing: 16) {
            Text("Question \(session.answeredCount + 1) of \(session.total)")
                .font(.footnote.weight(.semibold)).foregroundColor(.secondary).textCase(.uppercase)
            Text(q.promptZH).font(.custom("Songti SC", size: 22)).bold()
            ForEach(Array(q.options.enumerated()), id: \.offset) { idx, opt in
                Button {
                    selected = idx
                } label: {
                    Text(opt).font(.custom("Songti SC", size: 18))
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(12)
                        .background(RoundedRectangle(cornerRadius: 11)
                            .fill(selected == idx ? Color.jade.opacity(0.18) : Color.secondary.opacity(0.06)))
                        .overlay(RoundedRectangle(cornerRadius: 11)
                            .stroke(selected == idx ? Color.jade : Color.secondary.opacity(0.2),
                                    lineWidth: selected == idx ? 1.5 : 0.5))
                }
                .buttonStyle(.plain)
            }
            Spacer()
            Button {
                guard let s = selected else { return }
                session.answer(optionIndex: s)
                selected = nil
                if session.isComplete { learner.completeComprehension(session) }
            } label: {
                Text(session.answeredCount + 1 == session.total ? "Finish" : "Next")
                    .frame(maxWidth: .infinity)
            }
            .buttonStyle(.borderedProminent).tint(.cinnabar)
            .disabled(selected == nil)
        }
    }

    // MARK: - Seal / result (M8)

    private var resultScreen: some View {
        VStack(spacing: 18) {
            Spacer()
            if session.sealEarned {
                SealStamp(reduceMotion: reduceMotion)
                Text("Story sealed").font(.title2.weight(.semibold))
                Text("\(session.correctCount)/\(session.total) comprehension · \(story.tokenCount) words read")
                    .font(.footnote).foregroundColor(.secondary).multilineTextAlignment(.center)
                Text("Frontier words are now **learning** — P(known) updated for every word you read.")
                    .font(.caption).foregroundColor(.secondary).multilineTextAlignment(.center)
                    .padding(.horizontal, 24)
            } else {
                Image(systemName: "arrow.counterclockwise.circle")
                    .font(.system(size: 72)).foregroundColor(.secondary)
                Text("Not sealed yet").font(.title2.weight(.semibold))
                Text("\(session.correctCount)/\(session.total) correct — re-read and try again to earn the seal.")
                    .font(.footnote).foregroundColor(.secondary).multilineTextAlignment(.center)
                    .padding(.horizontal, 24)
            }
            Spacer()
            Button("Done", action: onDone).frame(maxWidth: .infinity)
                .buttonStyle(.borderedProminent).tint(.cinnabar)
        }
    }
}

/// The cinnabar seal stamp 读完为证 ("read to completion, hereby attested"). Presses in with a
/// rotation; a plain fade under Reduce Motion (NFR-6).
struct SealStamp: View {
    let reduceMotion: Bool
    @State private var shown = false

    var body: some View {
        RoundedRectangle(cornerRadius: 14)
            .fill(Color.cinnabar)
            .frame(width: 120, height: 120)
            .overlay(
                Text("读完\n为证").font(.custom("Songti SC", size: 30).weight(.heavy))
                    .foregroundColor(.white).multilineTextAlignment(.center))
            .overlay(RoundedRectangle(cornerRadius: 14)
                .strokeBorder(Color.white.opacity(0.9), lineWidth: 4).padding(3))
            .rotationEffect(.degrees(reduceMotion ? 0 : (shown ? -4 : 8)))
            .scaleEffect(reduceMotion ? 1 : (shown ? 1 : 1.4))
            .opacity(shown ? 1 : 0)
            .onAppear {
                if reduceMotion { shown = true }
                else { withAnimation(.spring(response: 0.4, dampingFraction: 0.55)) { shown = true } }
            }
            .accessibilityLabel("Story sealed")
    }
}
