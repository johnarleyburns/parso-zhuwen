import SwiftUI
import ZhuwenCore
import ZhuwenPacks
#if canImport(UIKit)
import UIKit
#elseif canImport(AppKit)
import AppKit
#endif

/// RootView is the app's tab shell (00 §10): Today · Library · Review · Progress. On first run it
/// auto-presents placement (FR-1.4); complete/partial beginners are routed into Foundations
/// (FR-11.6) until the F3 handoff fires (FR-11.5), when the regular loop activates.
public struct RootView: View {
    @ObservedObject private var model: AppModel
    @State private var selectedTab: AppTab = .today

    public init(model: AppModel) {
        self.model = model
    }

    public var body: some View {
        ZStack {
            Palette.backgroundGradient.ignoresSafeArea()

            switch model.onboardingRoute {
            case .needsPlacement:
                PlacementView(model: model.makePlacementFlow()) { result in
                    if let result { model.completePlacement(result) }
                }
                .accessibilityIdentifier("onboardingPlacement")
            case .foundations:
                FoundationsView(model: model.makeFoundationsModel()) { model.finishHandoff() }
                    .accessibilityIdentifier("foundationsCourse")
            case .loop:
                loop
            }
        }
    }

    private var loop: some View {
        VStack(spacing: 0) {
            ZStack {
                switch selectedTab {
                case .today:     TodayView(model: model)
                case .library:   LibraryView(model: model)
                case .review:    ReviewView(learner: model.learner)
                case .progress:  LearnerProgressView(learner: model.learner)
                }
            }
            .frame(maxHeight: .infinity)

            GlassTabBar(selection: $selectedTab)
        }
    }
}

/// Today: the engine-selected story with a hero cover card (M4/M11).
struct TodayView: View {
    @ObservedObject var model: AppModel
    @State private var showAttribution = false

    var body: some View {
        NavigationStack {
            Group {
                if let story = model.stories.first {
                    ScrollView {
                        VStack(spacing: 16) {
                            heroCover(story)
                            List {
                                Section("Today") {
                                    NavigationLink {
                                        ReaderView(model: model.readerModel(for: story),
                                                   listen: { model.makeListeningModel(for: story) },
                                                   comprehension: { model.makeComprehensionView(for: story) },
                                                   tapWord: { model.learner.lookup($0, storyID: story.id) })
                                    } label: {
                                        StoryRow(story: story, model: model)
                                    }
                                    .accessibilityIdentifier("todayStory")
                                }
                                if let m = model.manifest {
                                    Section("Pack") {
                                        LabeledContent("id", value: m.id)
                                        LabeledContent("lexicon", value: m.lexiconVersion)
                                        LabeledContent("stories", value: "\(model.stories.count)")
                                    }
                                }
                            }
                            .listStyle(.plain)
                            .frame(height: 300)
                        }
                    }
                } else {
                    ContentUnavailableViewCompat(title: "No pack loaded")
                }
            }
            .navigationTitle("Zhuwen")
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    NavigationLink { model.makeSettingsView() } label: {
                        Image(systemName: "gearshape")
                    }
                }
            }
            .sheet(isPresented: $showAttribution) {
                if let story = model.stories.first,
                   let rec = model.coverImage(for: story).record {
                    AttributionSheet(image: rec)
                }
            }
        }
    }

    @ViewBuilder
    private func heroCover(_ story: StoryRecord) -> some View {
        let cover = model.coverImage(for: story)
        ZStack {
            RoundedRectangle(cornerRadius: 22).fill(Color.secondary.opacity(0.08))
            if let data = cover.data, let image = Self.platformImage(data) {
                image.resizable().scaledToFill()
            } else {
                VStack(spacing: 8) {
                    Image(systemName: "book.closed").font(.system(size: 36)).foregroundColor(.secondary)
                    Text(story.titleZH).font(.custom("Songti SC", size: 28))
                        .foregroundColor(.primary)
                }
            }
        }
        .frame(height: 200)
        .clipShape(RoundedRectangle(cornerRadius: 22))
        .padding(.horizontal, 16)
        .padding(.top, 8)
        .glassSurface(cornerRadius: 22)
        .onLongPressGesture { showAttribution = true }
        .accessibilityLabel("Cover for \(story.titleZH)")
    }

    static func platformImage(_ data: Data) -> Image? {
        #if canImport(UIKit)
        if let ui = UIImage(data: data) { return Image(uiImage: ui) }
        #elseif canImport(AppKit)
        if let ns = NSImage(data: data) { return Image(nsImage: ns) }
        #endif
        return nil
    }
}

/// Library: browse every story with cover thumbnail + per-story new-word badge (FR-3.3).
struct LibraryView: View {
    @ObservedObject var model: AppModel

    var body: some View {
        NavigationStack {
            List(model.stories, id: \.id) { story in
                NavigationLink {
                    ReaderView(model: model.readerModel(for: story),
                               listen: { model.makeListeningModel(for: story) },
                               comprehension: { model.makeComprehensionView(for: story) },
                               tapWord: { model.learner.lookup($0, storyID: story.id) })
                } label: {
                    StoryRow(story: story, model: model)
                }
                .listRowBackground(Color.clear)
            }
            .scrollContentBackground(.hidden)
            .navigationTitle("Library")
        }
    }
}

@MainActor
struct StoryRow: View {
    let story: StoryRecord
    var model: AppModel? = nil
    @State private var showAttribution = false

    var body: some View {
        HStack(spacing: 12) {
            coverThumbnail
            VStack(alignment: .leading, spacing: 4) {
                Text(story.titleZH).font(.headline).foregroundColor(Palette.ink)
                HStack(spacing: 8) {
                    Text(story.band)
                    Text("· \(story.newTypeIDs.count) new")
                        .foregroundColor(Palette.cinnabar)
                    Text("· \(story.tokenCount) tokens")
                }
                .font(.caption)
                .foregroundColor(Palette.ink3)
            }
        }
        .padding(.vertical, 4)
        .sheet(isPresented: $showAttribution) {
            if let model = model, let rec = model.coverImage(for: story).record {
                AttributionSheet(image: rec)
            }
        }
    }

    @ViewBuilder
    private var coverThumbnail: some View {
        Group {
            if let model = model {
                let cover = model.coverImage(for: story)
                if let data = cover.data, let image = Self.platformImage(data) {
                    image.resizable().scaledToFill()
                } else {
                    coverPlaceholder
                }
            } else {
                coverPlaceholder
            }
        }
        .frame(width: 52, height: 52)
        .clipShape(RoundedRectangle(cornerRadius: 10))
        .onLongPressGesture { showAttribution = true }
    }

    private var coverPlaceholder: some View {
        ZStack {
            Color.secondary.opacity(0.12)
            Text(String(story.titleZH.prefix(1)))
                .font(.custom("Songti SC", size: 18))
                .foregroundColor(Palette.ink3)
        }
    }

    static func platformImage(_ data: Data) -> Image? {
        #if canImport(UIKit)
        if let ui = UIImage(data: data) { return Image(uiImage: ui) }
        #elseif canImport(AppKit)
        if let ns = NSImage(data: data) { return Image(nsImage: ns) }
        #endif
        return nil
    }
}

struct PlaceholderTab: View {
    let title: String
    let systemImage: String
    let note: String

    var body: some View {
        VStack(spacing: 8) {
            Image(systemName: systemImage).font(.largeTitle).foregroundColor(.secondary)
            Text(title).font(.headline)
            Text(note).font(.footnote).foregroundColor(.secondary)
        }
    }
}

/// Minimal stand-in so the shell compiles on macOS (ContentUnavailableView is iOS 17+).
struct ContentUnavailableViewCompat: View {
    let title: String
    var body: some View {
        Text(title).foregroundColor(.secondary)
    }
}
