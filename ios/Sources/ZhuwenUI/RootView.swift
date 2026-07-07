import SwiftUI
import ZhuwenCore
import ZhuwenPacks

/// RootView is the app's tab shell (00 §10): Today · Library · Review · Progress. On first run it
/// auto-presents placement (FR-1.4); complete/partial beginners are routed into Foundations
/// (FR-11.6) until the F3 handoff fires (FR-11.5), when the regular loop activates.
public struct RootView: View {
    @ObservedObject private var model: AppModel

    public init(model: AppModel) {
        self.model = model
    }

    public var body: some View {
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

    private var loop: some View {
        TabView {
            TodayView(model: model)
                .tabItem { Label("Today", systemImage: "book") }
            LibraryView(model: model)
                .tabItem { Label("Library", systemImage: "square.grid.2x2") }
            ReviewView(learner: model.learner)
                .tabItem { Label("Review", systemImage: "rectangle.stack") }
            LearnerProgressView(learner: model.learner)
                .tabItem { Label("Progress", systemImage: "chart.bar") }
        }
        .tint(.cinnabar)
    }
}

/// Today: the engine-selected story (CP-04 will do real selection; CP-03 shows the first).
struct TodayView: View {
    @ObservedObject var model: AppModel

    var body: some View {
        NavigationStack {
            Group {
                if let story = model.stories.first {
                    List {
                        Section("Today") {
                            NavigationLink {
                                ReaderView(model: model.readerModel(for: story),
                                           listen: { model.makeListeningModel(for: story) },
                                           comprehension: { model.makeComprehensionView(for: story) },
                                           tapWord: { model.learner.lookup($0, storyID: story.id) })
                            } label: {
                                StoryRow(story: story)
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
        }
    }
}

/// Library: browse every story with a per-story new-word badge (FR-3.3, partial).
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
                    StoryRow(story: story)
                }
            }
            .navigationTitle("Library")
        }
    }
}

struct StoryRow: View {
    let story: StoryRecord

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(story.titleZH).font(.headline)
            HStack(spacing: 8) {
                Text(story.band)
                Text("· \(story.newTypeIDs.count) new")
                    .foregroundColor(.cinnabar)
                Text("· \(story.tokenCount) tokens")
            }
            .font(.caption)
            .foregroundColor(.secondary)
        }
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
