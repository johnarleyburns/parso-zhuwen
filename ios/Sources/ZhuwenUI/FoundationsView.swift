import SwiftUI
import ZhuwenCore
import ZhuwenPacks
#if canImport(UIKit)
import UIKit
#elseif canImport(AppKit)
import AppKit
#endif

/// The Foundations picture-word course (M14, FR-11). A photo + audio + hanzi + tone-colored
/// pinyin card, driven through the F0 four-step cycle (introduce → recognize → read → bind), with
/// recognition grids, a long-press **attribution** sheet, and a **Credits** screen (FR-11.2). The
/// session closes on an F1/F2 recombination pass (FR-11.4); progress counts words from #1 and the
/// CEFR line reads "Pre-A1 · Foundations" until the F3 handoff (FR-11.5).
public struct FoundationsView: View {
    @ObservedObject private var model: FoundationsModel
    @State private var showAttribution = false
    @State private var showCredits = false
    private let onHandoff: () -> Void

    public init(model: FoundationsModel, onHandoff: @escaping () -> Void = {}) {
        self.model = model
        self.onHandoff = onHandoff
    }

    public var body: some View {
        NavigationStack {
            VStack(spacing: 16) {
                header
                if model.program.isEmpty {
                    ContentUnavailableViewCompat(title: "No Foundations content in this pack")
                    Spacer()
                } else if model.isSessionComplete {
                    recombinationScreen
                } else if let card = model.currentCard {
                    cardScreen(card)
                }
            }
            .padding(20)
            .navigationTitle("Foundations")
            .toolbar {
                ToolbarItem(placement: .primaryAction) {
                    Button { showCredits = true } label: { Image(systemName: "info.circle") }
                        .accessibilityLabel("Image credits")
                }
            }
            .sheet(isPresented: $showCredits) {
                CreditsView(images: model.program.allCards.compactMap { model.image(id: $0.imageID) })
            }
        }
    }

    // MARK: - Header (FR-11.5)

    private var header: some View {
        VStack(spacing: 6) {
            HStack {
                Text(model.currentSet?.name ?? "Foundations")
                    .font(.headline)
                Spacer()
                Text("Pre-A1 · Foundations")
                    .font(.caption.weight(.semibold))
                    .foregroundColor(.cinnabar)
            }
            HStack {
                Text("\(model.wordsKnown) words known").font(.caption).foregroundColor(.secondary)
                Spacer()
                Text("\(model.handoffStatus.storiesGated)/\(model.handoffStatus.threshold) stories unlockable")
                    .font(.caption).foregroundColor(.secondary)
            }
            ProgressView(value: Double(min(model.handoffStatus.storiesGated, model.handoffStatus.threshold)),
                         total: Double(model.handoffStatus.threshold))
                .tint(.jade)
        }
        .accessibilityIdentifier("foundationsHeader")
    }

    // MARK: - Card (M14)

    private func cardScreen(_ card: FoundationsCard) -> some View {
        VStack(spacing: 16) {
            photo(card)
                .onLongPressGesture { showAttribution = true }
                .sheet(isPresented: $showAttribution) {
                    if let rec = model.image(for: card).record { AttributionSheet(image: rec) }
                }

            VStack(spacing: 4) {
                Text(card.simp).font(.custom("Songti SC", size: 52)).bold()
                TonePinyinText(pinyin: card.pinyin).font(.title3)
            }

            Button {
                model.speak(card.simp)
            } label: {
                Label("Listen", systemImage: "speaker.wave.2.fill")
            }
            .buttonStyle(.bordered).tint(.cinnabar)
            .accessibilityIdentifier("foundationsListen")

            Spacer()
            interaction(card)
        }
    }

    @ViewBuilder
    private func photo(_ card: FoundationsCard) -> some View {
        let img = model.image(for: card)
        ZStack {
            RoundedRectangle(cornerRadius: 18).fill(Color.secondary.opacity(0.12))
            if let data = img.data, let image = Self.platformImage(data) {
                image.resizable().scaledToFill()
            } else {
                Image(systemName: "photo").font(.system(size: 44)).foregroundColor(.secondary)
            }
        }
        .frame(height: 220)
        .glassSurface(cornerRadius: 18)
        .accessibilityLabel(Text("Photo for \(card.simp)"))
        .accessibilityIdentifier("foundationsPhoto")
    }

    @ViewBuilder
    private func interaction(_ card: FoundationsCard) -> some View {
        switch model.currentStage {
        case .introduce:
            VStack(spacing: 6) {
                Text("Look, listen, and read. This is **\(card.simp)**.")
                    .font(.footnote).foregroundColor(.secondary).multilineTextAlignment(.center)
                Button { model.advance() } label: { Text("Got it").frame(maxWidth: .infinity) }
                    .buttonStyle(.borderedProminent).tint(.cinnabar)
                    .accessibilityIdentifier("foundationsIntroduceNext")
            }
        case .recognize:
            recognitionGrid(card, prompt: "Which picture is 「\(card.simp)」?", showPhotos: true)
        case .read:
            recognitionGrid(card, prompt: "Which word matches the photo?", showPhotos: false)
        case .bind:
            VStack(spacing: 6) {
                Text("Say it, then confirm the match.").font(.footnote).foregroundColor(.secondary)
                Button { model.advance(correct: true) } label: {
                    Text("I've got it").frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent).tint(.jade)
                .accessibilityIdentifier("foundationsBind")
            }
        case .none:
            EmptyView()
        }
    }

    private func recognitionGrid(_ card: FoundationsCard, prompt: String, showPhotos: Bool) -> some View {
        let options = model.gridOptions(for: card)
        return VStack(spacing: 10) {
            Text(prompt).font(.subheadline.weight(.semibold)).multilineTextAlignment(.center)
            LazyVGrid(columns: [GridItem(.flexible()), GridItem(.flexible())], spacing: 10) {
                ForEach(options) { opt in
                    Button {
                        model.advance(correct: opt.wordID == card.wordID)
                    } label: {
                        gridCell(opt, showPhoto: showPhotos)
                    }
                    .buttonStyle(.plain)
                    .accessibilityIdentifier(opt.wordID == card.wordID ? "foundationsCorrectOption" : "foundationsOption")
                }
            }
            if model.lastAnswerCorrect == false {
                Text("Not quite — listen again and pick the match.")
                    .font(.caption).foregroundColor(.cinnabar)
            }
        }
    }

    @ViewBuilder
    private func gridCell(_ card: FoundationsCard, showPhoto: Bool) -> some View {
        if showPhoto {
            let img = model.image(for: card)
            ZStack {
                RoundedRectangle(cornerRadius: 12).fill(Color.secondary.opacity(0.1))
                if let data = img.data, let image = Self.platformImage(data) {
                    image.resizable().scaledToFill()
                } else {
                    Image(systemName: "photo").foregroundColor(.secondary)
                }
            }
            .frame(height: 90).glassSurface(cornerRadius: 12)
        } else {
            Text(card.simp).font(.custom("Songti SC", size: 28))
                .frame(maxWidth: .infinity).frame(height: 64)
                .glassSurface(cornerRadius: 12)
        }
    }

    // MARK: - Recombination / handoff (FR-11.4 / F3)

    private var recombinationScreen: some View {
        VStack(spacing: 16) {
            Spacer()
            Image(systemName: "text.book.closed").font(.system(size: 56)).foregroundColor(.jade)
            Text("Set complete").font(.title2.weight(.semibold))
            Text("You read a little story using every word you just learned.")
                .font(.footnote).foregroundColor(.secondary).multilineTextAlignment(.center)
            Spacer()
            if model.isHandoffReady {
                Text("You're ready for real stories.").font(.headline)
                Button { onHandoff() } label: { Text("Start reading").frame(maxWidth: .infinity) }
                    .buttonStyle(.borderedProminent).tint(.cinnabar)
                    .accessibilityIdentifier("foundationsHandoff")
            } else if model.hasNextSet {
                Button { model.startNextSet() } label: { Text("Next set").frame(maxWidth: .infinity) }
                    .buttonStyle(.borderedProminent).tint(.cinnabar)
                    .accessibilityIdentifier("foundationsNextSet")
            } else {
                Text("More Foundations content arrives with the next pack.")
                    .font(.footnote).foregroundColor(.secondary)
            }
        }
    }

    // MARK: - Cross-platform image decode

    static func platformImage(_ data: Data) -> Image? {
        #if canImport(UIKit)
        if let ui = UIImage(data: data) { return Image(uiImage: ui) }
        #elseif canImport(AppKit)
        if let ns = NSImage(data: data) { return Image(nsImage: ns) }
        #endif
        return nil
    }
}

// MARK: - Tone-colored pinyin

/// Renders pinyin with the standard five-tone color scheme (tone detected from the diacritic).
/// A modest realization of M14's "tone-colored pinyin"; unknown/neutral tones render secondary.
struct TonePinyinText: View {
    let pinyin: String

    var body: some View {
        HStack(spacing: 4) {
            ForEach(Array(pinyin.split(separator: " ").enumerated()), id: \.offset) { _, syl in
                Text(String(syl)).foregroundColor(Self.color(for: String(syl)))
            }
        }
    }

    static func color(for syllable: String) -> Color {
        switch tone(of: syllable) {
        case 1: return Color(red: 0.86, green: 0.15, blue: 0.15) // high — red
        case 2: return Color(red: 0.90, green: 0.55, blue: 0.10) // rising — orange
        case 3: return Color(red: 0.16, green: 0.49, blue: 0.36) // low-dip — green
        case 4: return Color(red: 0.20, green: 0.35, blue: 0.75) // falling — blue
        default: return .secondary                               // neutral
        }
    }

    static func tone(of syllable: String) -> Int {
        let marks: [Character: Int] = [
            "ā": 1, "ē": 1, "ī": 1, "ō": 1, "ū": 1, "ǖ": 1,
            "á": 2, "é": 2, "í": 2, "ó": 2, "ú": 2, "ǘ": 2,
            "ǎ": 3, "ě": 3, "ǐ": 3, "ǒ": 3, "ǔ": 3, "ǚ": 3,
            "à": 4, "è": 4, "ì": 4, "ò": 4, "ù": 4, "ǜ": 4,
        ]
        for c in syllable.lowercased() { if let t = marks[c] { return t } }
        // Trailing tone number (pinyin like "ci2").
        if let last = syllable.last, let n = last.wholeNumberValue, (0...4).contains(n) { return n }
        return 0
    }
}

// MARK: - Attribution (FR-11.2)

/// The long-press attribution sheet: author, license, and a link to the Commons source (the
/// CC-BY/SA legal obligation, FR-11.2). Never editable; provenance ships inside the pack.
struct AttributionSheet: View {
    let image: ImageRecord
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            List {
                LabeledContent("Author", value: image.author.isEmpty ? "—" : image.author)
                LabeledContent("License", value: image.license)
                if let url = URL(string: image.licenseURL), !image.licenseURL.isEmpty {
                    Link("License terms", destination: url)
                }
                if let url = URL(string: image.sourceURL), !image.sourceURL.isEmpty {
                    Link("Source on Wikimedia Commons", destination: url)
                }
            }
            .scrollContentBackground(.hidden)
            .adaptiveGlass(cornerRadius: 20)
            .navigationTitle("Image credit")
            .toolbar {
                ToolbarItem(placement: .confirmationAction) { Button("Done") { dismiss() } }
            }
        }
    }
}

// MARK: - Credits (FR-11.2)

/// The full image credits screen: every Commons photo the app ships, with author, license, and
/// source link — the CC-BY/SA attribution obligation for the whole Foundations inventory.
public struct CreditsView: View {
    private let images: [ImageRecord]
    @Environment(\.dismiss) private var dismiss

    public init(images: [ImageRecord]) {
        // De-duplicate by id and order for a stable list.
        var seen = Set<String>()
        self.images = images.filter { seen.insert($0.id).inserted }.sorted { $0.id < $1.id }
    }

    public var body: some View {
        NavigationStack {
            List {
                Section {
                    Text("Every photograph in Zhuwen is a real photograph or public-domain artwork sourced from Wikimedia Commons. No image is AI-generated (product-wide).")
                        .font(.footnote).foregroundColor(.secondary)
                }
                ForEach(images) { image in
                    VStack(alignment: .leading, spacing: 4) {
                        Text(image.author.isEmpty ? image.id : image.author).font(.subheadline.weight(.semibold))
                        Text(image.license).font(.caption).foregroundColor(.secondary)
                        if let url = URL(string: image.sourceURL), !image.sourceURL.isEmpty {
                            Link("Source", destination: url).font(.caption)
                        }
                    }
                }
            }
            .navigationTitle("Image credits")
            .toolbar {
                ToolbarItem(placement: .confirmationAction) { Button("Done") { dismiss() } }
            }
        }
    }
}
