import SwiftUI
import ZhuwenCore

/// The paywall (M12, FR-9.4): a single, factual, **dismissible** screen. Copy is honest — the free
/// tier is the full method at one story/day; Pro removes the throttle and opens the lattice. It never
/// interrupts an in-progress story (callers present it only from Pro-gated affordances). No ads,
/// StoreKit 2 only (FR-9.3).
public struct PaywallView: View {
    @ObservedObject private var store: StoreModel
    @Environment(\.dismiss) private var dismiss
    @State private var busy = false

    public init(store: StoreModel) {
        self.store = store
    }

    public var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 20) {
                    header
                    valueProps
                    ForEach(store.products) { product in
                        planButton(product)
                    }
                    Button("Restore Purchases") {
                        Task { busy = true; await store.restore(); busy = false }
                    }
                    .font(.footnote)
                    .frame(maxWidth: .infinity)

                    Text("The free tier is the full method — placement, Foundations, one engine-selected story a day, dictionary, and review. Zhuwen Pro removes the daily limit and opens the whole story lattice, listening packs, and the full dashboard. No ads, ever.")
                        .font(.footnote)
                        .foregroundColor(.secondary)

                    if let error = store.lastError {
                        Text(error).font(.caption).foregroundColor(.cinnabar)
                    }
                }
                .padding(20)
            }
            .navigationTitle("Zhuwen Pro")
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Not now") { dismiss() }
                }
            }
            .disabled(busy)
        }
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Unlimited reading, the whole lattice")
                .font(.title2.bold())
            Text("Keep the method. Lose the daily limit.")
                .font(.subheadline).foregroundColor(.secondary)
        }
    }

    private var valueProps: some View {
        VStack(alignment: .leading, spacing: 10) {
            prop("infinity", "Unlimited stories — read as much as you want")
            prop("square.grid.3x3", "Browse the full story lattice")
            prop("headphones", "Listening packs & blind mode")
            prop("chart.bar.doc.horizontal", "Full dashboard & monthly checkpoints")
            prop("shippingbox", "Every future pack included")
        }
    }

    private func prop(_ icon: String, _ text: String) -> some View {
        HStack(spacing: 10) {
            Image(systemName: icon).foregroundColor(.cinnabar).frame(width: 24)
            Text(text).font(.subheadline)
        }
    }

    private func planButton(_ product: StoreProduct) -> some View {
        Button {
            Task { busy = true; await store.purchase(product); busy = false
                if store.isPro { dismiss() } }
        } label: {
            HStack {
                VStack(alignment: .leading, spacing: 2) {
                    Text(planTitle(product)).font(.headline)
                    if product.trialDays > 0 {
                        Text("\(product.trialDays)-day free trial")
                            .font(.caption).foregroundColor(.jade)
                    }
                }
                Spacer()
                Text(product.displayPrice).font(.headline)
            }
            .padding()
            .frame(maxWidth: .infinity)
            .background(RoundedRectangle(cornerRadius: 12)
                .fill(product.kind == .annual ? Color.cinnabar.opacity(0.12) : Color.secondary.opacity(0.08)))
            .overlay(RoundedRectangle(cornerRadius: 12)
                .stroke(product.kind == .annual ? Color.cinnabar : Color.clear, lineWidth: 1.5))
        }
        .buttonStyle(.plain)
    }

    private func planTitle(_ product: StoreProduct) -> String {
        switch product.kind {
        case .monthly: return "Monthly"
        case .annual: return "Annual"
        case .lifetime: return "Lifetime"
        }
    }
}
