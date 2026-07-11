import SwiftUI

enum AppTab: String, CaseIterable {
    case today = "Today"
    case library = "Library"
    case review = "Review"
    case progress = "Progress"
}

struct GlassTabBar: View {
    @Binding var selection: AppTab

    private let items: [(AppTab, String, String)] = [
        (.today, "今", "Today"),
        (.library, "书", "Library"),
        (.review, "忆", "Review"),
        (.progress, "进", "Progress")
    ]

    var body: some View {
        HStack(spacing: 0) {
            ForEach(items, id: \.0) { tab, hanzi, label in
                Button {
                    selection = tab
                } label: {
                    VStack(spacing: 2) {
                        Text(hanzi)
                            .font(.custom("Songti SC", size: 20)).bold()
                        Text(label)
                            .font(.system(size: 9.5, weight: .medium))
                    }
                    .foregroundStyle(selection == tab ? Palette.gold : Palette.ink3)
                    .frame(maxWidth: .infinity)
                }
                .accessibilityLabel(label)
                .accessibilityAddTraits(selection == tab ? [.isSelected] : [])
            }
        }
        .padding(.vertical, 10)
        .padding(.horizontal, 6)
        .adaptiveGlass(cornerRadius: 24)
        .padding(.horizontal, 16)
        .padding(.bottom, 8)
    }
}
