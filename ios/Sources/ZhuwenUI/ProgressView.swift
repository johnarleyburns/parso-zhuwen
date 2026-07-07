import SwiftUI
import ZhuwenCore

/// The progress dashboard (M10, FR-6.3). Separate reading and listening band estimates, lexicon
/// growth, HSK-3.0 gap, and a CEFR can-do line. Everything is labeled an estimate (I4/FR-6.3).
public struct LearnerProgressView: View {
    @ObservedObject private var learner: LearnerModel

    public init(learner: LearnerModel) {
        self.learner = learner
    }

    public var body: some View {
        let r = learner.progress()
        return NavigationStack {
            ScrollView {
                VStack(spacing: 12) {
                    HStack(spacing: 12) {
                        BandCard(title: "Reading", band: r.readingBand == .a0 ? "Pre-A1" : r.readingLabel,
                                 progress: r.readingProgressToNext, tint: .cinnabar,
                                 note: r.readingBand == .a0 ? "Foundations" : nil)
                        BandCard(title: "Listening", band: r.listeningLabel,
                                 progress: r.listeningProgressToNext, tint: .secondary)
                    }
                    lexiconCard(r)
                    hskCard(r)
                    canDoCard(r)
                }
                .padding(16)
            }
            .navigationTitle("Progress")
        }
    }

    private func lexiconCard(_ r: ProgressReport) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(alignment: .firstTextBaseline) {
                Text("\(r.wordsKnown)").font(.system(size: 30, weight: .heavy))
                Text("words known").font(.footnote).foregroundColor(.secondary)
                Spacer()
                Text("+\(r.wordsKnownThisWeek) this week")
                    .font(.caption.weight(.semibold)).foregroundColor(.jade)
            }
            GrowthBars(series: r.weeklyKnownSeries).frame(height: 60)
            HStack {
                Text("\(r.weeklyKnownSeries.count) weeks ago"); Spacer(); Text("now")
            }
            .font(.caption2).foregroundColor(.secondary)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(16)
        .background(RoundedRectangle(cornerRadius: 14).fill(Color.secondary.opacity(0.08)))
    }

    private func hskCard(_ r: ProgressReport) -> some View {
        HStack {
            VStack(alignment: .leading, spacing: 2) {
                Text("HSK 3.0 · Level \(r.hskLevel)").font(.subheadline.weight(.semibold))
                Text("\(r.wordsToNextHSK) words to Level \(r.hskLevel + 1) coverage")
                    .font(.caption).foregroundColor(.secondary)
            }
            Spacer()
            Image(systemName: "chevron.right").foregroundColor(.secondary)
        }
        .padding(16)
        .background(RoundedRectangle(cornerRadius: 14).fill(Color.secondary.opacity(0.08)))
    }

    private func canDoCard(_ r: ProgressReport) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("You can now…").font(.footnote.weight(.semibold))
                .foregroundColor(.secondary).textCase(.uppercase)
            Text("\(r.canDo) (CEFR \(r.readingLabel) can-do · estimate)")
                .font(.subheadline).foregroundColor(.secondary)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(16)
        .background(RoundedRectangle(cornerRadius: 14).fill(Color.secondary.opacity(0.08)))
    }
}

private struct BandCard: View {
    let title: String
    let band: String
    let progress: Double
    let tint: Color
    var note: String? = nil

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(title).font(.footnote.weight(.semibold)).foregroundColor(.secondary).textCase(.uppercase)
            Text(band).font(.system(size: 32, weight: .heavy)).foregroundColor(tint)
            if let note {
                Text(note).font(.caption2.weight(.semibold)).foregroundColor(.cinnabar)
            }
            ProgressView(value: min(1, max(0, progress))).tint(tint)
            Text("\(Int((progress * 100).rounded()))% to next · est.")
                .font(.caption2).foregroundColor(.secondary)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(14)
        .background(RoundedRectangle(cornerRadius: 14).fill(Color.secondary.opacity(0.08)))
    }
}

private struct GrowthBars: View {
    let series: [Int]

    var body: some View {
        GeometryReader { geo in
            let maxV = max(1, series.max() ?? 1)
            HStack(alignment: .bottom, spacing: 4) {
                ForEach(Array(series.enumerated()), id: \.offset) { i, v in
                    RoundedRectangle(cornerRadius: 3)
                        .fill(Color.jade.opacity(0.4 + 0.6 * Double(i) / Double(max(1, series.count - 1))))
                        .frame(height: max(2, CGFloat(Double(v) / Double(maxV)) * geo.size.height))
                        .frame(maxWidth: .infinity)
                }
            }
        }
    }
}
