import SwiftUI
import ZhuwenCore

/// The FSRS review screen (M9, FR-7.1/7.2). Sentence-context cards only — the word appears inside a
/// sentence the learner read, cited to its source story. Grade buttons show projected intervals;
/// grading feeds the known-word model (FR-7.3). Capped by the scheduler (20/day default, FR-7.2).
public struct ReviewView: View {
    @ObservedObject private var learner: LearnerModel
    @State private var queue: [ReviewCard]
    @State private var index = 0
    @State private var revealed = false

    public init(learner: LearnerModel) {
        self.learner = learner
        _queue = State(initialValue: learner.reviewQueue())
    }

    public var body: some View {
        NavigationStack {
            Group {
                if index < queue.count {
                    card(queue[index])
                } else {
                    done
                }
            }
            .navigationTitle("Review")
            .toolbar {
                if index < queue.count {
                    Text("\(queue.count - index) due").font(.subheadline.weight(.semibold))
                        .foregroundColor(.secondary)
                }
            }
        }
    }

    private func card(_ c: ReviewCard) -> some View {
        VStack(spacing: 16) {
            VStack(spacing: 18) {
                Text("From “\(c.storyTitleZH)”")
                    .font(.footnote.weight(.semibold)).foregroundColor(.secondary).textCase(.uppercase)
                sentence(c).font(.custom("Songti SC", size: 24))
                if revealed {
                    Divider()
                    Text(c.pinyin).font(.title3).foregroundColor(.cinnabar)
                    Text("HSK \(c.hsk3Level) · rank shown in reader")
                        .font(.footnote).foregroundColor(.secondary)
                }
            }
            .frame(maxWidth: .infinity)
            .padding(22)
            .glassSurface(cornerRadius: 14)

            Spacer()

            if revealed {
                HStack(spacing: 8) {
                    gradeButton(c, .again, Color(red: 0.71, green: 0.27, blue: 0.24))
                    gradeButton(c, .hard, Color(red: 0.73, green: 0.54, blue: 0.18))
                    gradeButton(c, .good, .jade)
                    gradeButton(c, .easy, Color(red: 0.24, green: 0.49, blue: 0.69))
                }
            } else {
                Text("Did you remember it before revealing?")
                    .font(.footnote).foregroundColor(.secondary)
                Button { revealed = true } label: {
                    Text("Reveal").frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent).tint(.cinnabar)
            }
        }
        .padding(20)
    }

    private func sentence(_ c: ReviewCard) -> Text {
        c.sentence.reduce(Text("")) { acc, tok in
            acc + Text(tok.text).foregroundColor(tok.isTarget ? .cinnabar : .primary)
                .fontWeight(tok.isTarget ? .bold : .regular)
        }
    }

    private func gradeButton(_ c: ReviewCard, _ rating: Rating, _ color: Color) -> some View {
        Button {
            learner.grade(c, rating)
            advance()
        } label: {
            VStack(spacing: 2) {
                Text(rating.label).font(.subheadline.weight(.semibold))
                Text(intervalLabel(c.intervals[rating] ?? 1)).font(.caption2).opacity(0.85)
            }
            .frame(maxWidth: .infinity).padding(.vertical, 12)
            .background(RoundedRectangle(cornerRadius: 12).fill(color))
            .foregroundColor(.white)
        }
        .buttonStyle(.plain)
    }

    private func intervalLabel(_ days: Int) -> String {
        days < 1 ? "<1 d" : (days == 1 ? "1 d" : "\(days) d")
    }

    private func advance() {
        revealed = false
        index += 1
    }

    private var done: some View {
        VStack(spacing: 10) {
            Image(systemName: "checkmark.seal.fill").font(.system(size: 56)).foregroundColor(.jade)
            Text(queue.isEmpty ? "Nothing due" : "Review complete").font(.headline)
            Text("Reviews are optional maintenance — the real repetition is meeting words again in stories.")
                .font(.footnote).foregroundColor(.secondary).multilineTextAlignment(.center)
                .padding(.horizontal, 32)
        }
        .padding()
    }
}
