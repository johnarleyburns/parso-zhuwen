import SwiftUI

struct AdaptiveGlass: ViewModifier {
    var cornerRadius: CGFloat = 26

    @Environment(\.accessibilityReduceTransparency) private var reduceTransparency

    func body(content: Content) -> some View {
        if reduceTransparency {
            content.background(
                RoundedRectangle(cornerRadius: cornerRadius, style: .continuous)
                    .fill(Palette.opaqueFallback)
                    .overlay(RoundedRectangle(cornerRadius: cornerRadius, style: .continuous)
                        .strokeBorder(Color.white.opacity(0.16), lineWidth: 1))
            )
        } else {
            content.background(
                RoundedRectangle(cornerRadius: cornerRadius, style: .continuous)
                    .fill(.ultraThinMaterial)
                    .overlay(RoundedRectangle(cornerRadius: cornerRadius, style: .continuous)
                        .fill(Palette.glassFill))
                    .overlay(RoundedRectangle(cornerRadius: cornerRadius, style: .continuous)
                        .strokeBorder(Color.white.opacity(0.16), lineWidth: 1))
                    .shadow(color: .black.opacity(0.45), radius: 18, y: 10)
            )
        }
    }
}

extension View {
    func adaptiveGlass(cornerRadius: CGFloat = 26) -> some View {
        modifier(AdaptiveGlass(cornerRadius: cornerRadius))
    }
}
