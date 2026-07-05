import SwiftUI
import UniformTypeIdentifiers
import ZhuwenCore
import ZhuwenPacks

/// SettingsView (M13, FR-10): reader/audio preferences (FR-10.1), the pack manager (FR-8.3), the
/// optional-and-off-by-default iCloud sync toggle (FR-10.2), Export / Erase (FR-10.3), a Pro row, and
/// the plain-language privacy page that states the I2 network-surface guarantee.
public struct SettingsView: View {
    @ObservedObject private var learner: LearnerModel
    @ObservedObject private var store: StoreModel
    @ObservedObject private var sync: SyncModel
    @ObservedObject private var packs: PackManagerModel
    @Binding private var settings: LearnerSettings

    @State private var showPaywall = false
    @State private var showEraseConfirm = false
    @State private var exportDocument: JSONDocument?
    @State private var showExporter = false

    public init(learner: LearnerModel, store: StoreModel, sync: SyncModel,
                packs: PackManagerModel, settings: Binding<LearnerSettings>) {
        self.learner = learner
        self.store = store
        self.sync = sync
        self.packs = packs
        _settings = settings
    }

    public var body: some View {
        NavigationStack {
            Form {
                subscriptionSection
                readingSection
                audioReviewSection
                packsSection
                syncSection
                dataSection
                privacySection
            }
            .navigationTitle("Settings")
            .sheet(isPresented: $showPaywall) { PaywallView(store: store) }
            .fileExporter(isPresented: $showExporter,
                          document: exportDocument,
                          contentType: .json,
                          defaultFilename: "zhuwen-export") { _ in }
            .alert("Erase everything?", isPresented: $showEraseConfirm) {
                Button("Erase", role: .destructive) { learner.eraseAll() }
                Button("Cancel", role: .cancel) {}
            } message: {
                Text("This permanently deletes your entire learning history on this device. Export first if you want a backup.")
            }
        }
    }

    // MARK: - Sections

    private var subscriptionSection: some View {
        Section("Subscription") {
            HStack {
                Text(store.isPro ? "Zhuwen Pro" : "Free")
                Spacer()
                if !store.isPro {
                    Button("Upgrade") { showPaywall = true }
                        .buttonStyle(.borderedProminent).tint(.cinnabar)
                } else {
                    Image(systemName: "checkmark.seal.fill").foregroundColor(.jade)
                }
            }
        }
    }

    private var readingSection: some View {
        Section("Reading") {
            Picker("Pinyin", selection: $settings.pinyinMode) {
                Text("Always").tag(LearnerSettings.PinyinMode.always)
                Text("Frontier only").tag(LearnerSettings.PinyinMode.frontierOnly)
                Text("On tap").tag(LearnerSettings.PinyinMode.onTap)
                Text("Never").tag(LearnerSettings.PinyinMode.never)
            }
            Toggle("Underline frontier words", isOn: $settings.frontierUnderline)
            Picker("Theme", selection: $settings.theme) {
                Text("System").tag(LearnerSettings.Theme.system)
                Text("Light").tag(LearnerSettings.Theme.light)
                Text("Sepia").tag(LearnerSettings.Theme.sepia)
                Text("Dark").tag(LearnerSettings.Theme.dark)
            }
            Stepper("Font size: \(Int(settings.readerFontSize)) pt",
                    value: $settings.readerFontSize, in: 14...32)
        }
    }

    private var audioReviewSection: some View {
        Section("Audio & review") {
            Picker("Voice", selection: $settings.audioVoice) {
                Text("Pack narration").tag(LearnerSettings.AudioVoice.pack)
                Text("System TTS").tag(LearnerSettings.AudioVoice.systemTTS)
            }
            Stepper("Daily review cap: \(settings.dailyReviewCap)",
                    value: $settings.dailyReviewCap, in: 5...50, step: 5)
        }
    }

    private var packsSection: some View {
        Section("Packs") {
            if packs.installed.isEmpty {
                Text("Starter pack embedded").foregroundColor(.secondary)
            }
            ForEach(packs.installed) { pack in
                HStack {
                    Text(pack.id)
                    Spacer()
                    Text(PackManagerModel.sizeLabel(pack.sizeBytes))
                        .foregroundColor(.secondary)
                    Button(role: .destructive) { packs.delete(pack) } label: {
                        Image(systemName: "trash")
                    }
                    .buttonStyle(.borderless)
                }
            }
            if packs.canDownload {
                NavigationLink("Browse pack library") {
                    PackLibraryView(packs: packs, gate: store.gate)
                }
            }
        }
    }

    private var syncSection: some View {
        Section {
            Toggle("iCloud sync", isOn: $sync.enabled)
                .disabled(!sync.isAvailable)
            if let at = sync.lastSyncedAt {
                Text("Last synced \(at.formatted())").font(.caption).foregroundColor(.secondary)
            }
        } header: {
            Text("Sync")
        } footer: {
            Text("Off by default. When on, your learning history syncs through your private iCloud account only — never to us. Content is never uploaded.")
        }
    }

    private var dataSection: some View {
        Section("Your data") {
            Button("Export everything (JSON)") {
                if let data = try? learner.exportJSON() {
                    exportDocument = JSONDocument(data: data)
                    showExporter = true
                }
            }
            Button("Erase everything", role: .destructive) { showEraseConfirm = true }
        }
    }

    private var privacySection: some View {
        Section("Privacy") {
            NavigationLink("Network & privacy") { PrivacyView() }
        }
    }
}

/// The plain-language privacy page (FR-10.3): states the I2 network-surface guarantee.
struct PrivacyView: View {
    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 14) {
                Text("What leaves your device").font(.headline)
                bullet("Anonymous pack downloads from our content CDN — no account, no identifiers.")
                bullet("Apple’s App Store, only when you purchase or restore Zhuwen Pro.")
                bullet("Your private iCloud, only if you turn on sync — and only your learning history, never content.")
                Divider()
                Text("What never leaves").font(.headline)
                bullet("No analytics. No crash SDKs. No ad networks. No tracking.")
                bullet("All learning happens on device. Your history is yours; export or erase it any time.")
            }
            .padding(20)
        }
        .navigationTitle("Network & privacy")
    }

    private func bullet(_ text: String) -> some View {
        HStack(alignment: .top, spacing: 8) {
            Text("•")
            Text(text)
        }
        .font(.subheadline)
    }
}

/// The downloadable pack library (FR-8.2/8.3). Non-Pro learners see the paywall gate on all-packs.
struct PackLibraryView: View {
    @ObservedObject var packs: PackManagerModel
    let gate: FeatureGate

    var body: some View {
        List {
            ForEach(packs.available) { pack in
                HStack {
                    VStack(alignment: .leading) {
                        Text(pack.id)
                        Text("\(pack.band) · \(PackManagerModel.sizeLabel(pack.sizeBytes))")
                            .font(.caption).foregroundColor(.secondary)
                    }
                    Spacer()
                    if packs.busyPackID == pack.id {
                        ProgressView()
                    } else {
                        Button("Get") { Task { await packs.download(pack) } }
                            .disabled(!gate.canDownloadAllPacks)
                    }
                }
            }
        }
        .navigationTitle("Pack library")
        .task { await packs.loadCatalog() }
    }
}

/// A tiny JSON document wrapper for `fileExporter` (FR-10.3 export).
struct JSONDocument: FileDocument {
    static var readableContentTypes: [UTType] { [.json] }
    var data: Data
    init(data: Data) { self.data = data }
    init(configuration: ReadConfiguration) throws {
        data = configuration.file.regularFileContents ?? Data()
    }
    func fileWrapper(configuration: WriteConfiguration) throws -> FileWrapper {
        FileWrapper(regularFileWithContents: data)
    }
}
