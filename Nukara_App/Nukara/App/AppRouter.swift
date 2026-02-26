import SwiftUI

@MainActor
struct RootView: View {
    @EnvironmentObject private var sessionStore: SessionStore
    @State private var selectedTab: Int = 0

    var body: some View {
        Group {
            if sessionStore.session == nil {
                LandingView()
            } else {
                TabView(selection: $selectedTab) {
                    ConversationsView()
                        .tabItem {
                            Label("消息", systemImage: "message")
                        }
                        .tag(0)

                    BotListView()
                        .tabItem {
                            Label("通讯录", systemImage: "person.2")
                        }
                        .tag(1)

                    ProfileView()
                        .tabItem {
                            Label("我的", systemImage: "person.crop.circle")
                        }
                        .tag(2)
                }
                .tint(Color.nkAccent)
            }
        }
        .task {
            await sessionStore.restoreSessionIfNeeded()
        }
    }
}

@MainActor
private struct ProfileView: View {
    @Environment(\.appEnvironment) private var appEnvironment
    @EnvironmentObject private var sessionStore: SessionStore
    @AppStorage("nukara.user.status.emoji") private var statusEmoji: String = "❔"
    @AppStorage("nukara.user.status.text") private var statusText: String = "未知"

    var body: some View {
        NavigationStack {
            VStack(spacing: 16) {
                VStack(spacing: 12) {
                    HStack(alignment: .top, spacing: 14) {
                        AvatarView(name: sessionStore.session?.user.nickname ?? "我", imageData: nil, size: 62)

                        VStack(alignment: .leading, spacing: 6) {
                            Text(sessionStore.session?.user.nickname ?? "未登录")
                                .font(.headline)
                                .foregroundStyle(Color.nkPrimaryText)

                            Text("ID: \(sessionStore.session?.user.id ?? "-")")
                                .font(.caption)
                                .foregroundStyle(Color.nkTextSecondary)

                            Text("账号: \(sessionStore.session?.user.nickname ?? "-")")
                                .font(.caption)
                                .foregroundStyle(Color.nkTextSecondary)

                            HStack(spacing: 6) {
                                Text("状态:")
                                    .font(.caption)
                                    .foregroundStyle(Color.nkTextSecondary)
                                Text("\(statusEmoji)\(statusText)")
                                    .font(.caption)
                                    .foregroundStyle(Color.nkPrimaryText)
                                NavigationLink {
                                    StatusEditorView(
                                        emoji: statusEmoji,
                                        text: statusText
                                    ) { [appEnvironment] emoji, text in
                                        statusEmoji = emoji
                                        statusText = text
                                        Task { await appEnvironment.userStatusService.syncStatus(emoji: emoji, text: text) }
                                    }
                                } label: {
                                    Image(systemName: "chevron.right")
                                        .font(.caption2.weight(.semibold))
                                        .foregroundStyle(Color.nkTextMuted)
                                }
                            }
                        }

                        Spacer()
                    }
                }
                .padding(16)
                .background(
                    RoundedRectangle(cornerRadius: 18, style: .continuous)
                        .fill(Color.nkCardBackground)
                )
                .padding(.horizontal, 16)

                VStack(alignment: .leading, spacing: 10) {
                    HStack {
                        Image(systemName: "terminal.fill")
                            .foregroundStyle(Color.nkAccent)
                        Text("环境")
                            .font(.subheadline)
                            .foregroundStyle(Color.nkPrimaryText)
                        Spacer()
                        Text(appEnvironment.mode.rawValue.uppercased())
                            .font(.caption)
                            .foregroundStyle(Color.nkTextSecondary)
                    }
                }
                .padding(16)
                .background(
                    RoundedRectangle(cornerRadius: 18, style: .continuous)
                        .fill(Color.nkCardBackground)
                )
                .padding(.horizontal, 16)

                Spacer()

                NavigationLink {
                    SettingsView {
                        Task { await sessionStore.logout() }
                    }
                } label: {
                    HStack {
                        Image(systemName: "gearshape.fill")
                        Text("设置")
                            .fontWeight(.semibold)
                        Spacer()
                        Image(systemName: "chevron.right")
                            .font(.caption)
                    }
                    .foregroundStyle(Color.nkTextOnAccent)
                    .padding(.horizontal, 16)
                    .padding(.vertical, 14)
                    .background(
                        RoundedRectangle(cornerRadius: 14, style: .continuous)
                            .fill(Color.nkAccent)
                    )
                }
                .padding(.horizontal, 16)
                .padding(.bottom, 14)
            }
            .background(Color.nkBackground.ignoresSafeArea())
            .navigationTitle("我的")
            .navigationBarTitleDisplayMode(.inline)
        }
    }
}

@MainActor
private struct StatusEditorView: View {
    @Environment(\.dismiss) private var dismiss

    @State private var selectedEmoji: String
    @State private var statusText: String

    let onSave: (String, String) -> Void

    private struct StatusOption: Identifiable {
        let emoji: String
        let text: String
        var id: String { "\(emoji)-\(text)" }
    }

    private struct StatusGroup: Identifiable {
        let title: String
        let options: [StatusOption]
        var id: String { title }
    }

    private let groupedStatuses: [StatusGroup] = [
        StatusGroup(title: "心情", options: [
            StatusOption(emoji: "🙂", text: "平静"),
            StatusOption(emoji: "🥳", text: "开心"),
            StatusOption(emoji: "😶‍🌫️", text: "放空"),
            StatusOption(emoji: "😴", text: "困了")
        ]),
        StatusGroup(title: "活动", options: [
            StatusOption(emoji: "🎧", text: "听歌"),
            StatusOption(emoji: "📚", text: "学习"),
            StatusOption(emoji: "🍜", text: "吃饭"),
            StatusOption(emoji: "🏃", text: "运动")
        ]),
        StatusGroup(title: "工作与生活", options: [
            StatusOption(emoji: "💻", text: "工作"),
            StatusOption(emoji: "☕️", text: "忙碌"),
            StatusOption(emoji: "🌙", text: "休息"),
            StatusOption(emoji: "✈️", text: "出行")
        ])
    ]

    init(emoji: String, text: String, onSave: @escaping (String, String) -> Void) {
        _selectedEmoji = State(initialValue: emoji)
        _statusText = State(initialValue: text)
        self.onSave = onSave
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                ForEach(groupedStatuses) { group in
                    VStack(alignment: .leading, spacing: 10) {
                        Text(group.title)
                            .font(.headline)
                            .foregroundStyle(Color.nkPrimaryText)

                        LazyVGrid(columns: [GridItem(.adaptive(minimum: 90), spacing: 10)], spacing: 10) {
                            ForEach(group.options) { item in
                                Button {
                                    selectedEmoji = item.emoji
                                    statusText = item.text
                                } label: {
                                    VStack(spacing: 4) {
                                        Text(item.emoji)
                                            .font(.title3)
                                        Text(item.text)
                                            .font(.caption)
                                            .foregroundStyle(Color.nkPrimaryText)
                                    }
                                    .frame(maxWidth: .infinity)
                                    .padding(.vertical, 10)
                                    .background(
                                        RoundedRectangle(cornerRadius: 12, style: .continuous)
                                            .fill(
                                                (selectedEmoji == item.emoji && statusText == item.text)
                                                ? Color.nkAccent.opacity(0.55)
                                                : Color.nkCardBackground
                                            )
                                    )
                                }
                            }
                        }
                    }
                }

                VStack(alignment: .leading, spacing: 8) {
                    Text("自定义状态")
                        .font(.headline)
                        .foregroundStyle(Color.nkPrimaryText)

                    HStack(spacing: 10) {
                        TextField("🙂", text: $selectedEmoji)
                            .frame(width: 52)
                            .multilineTextAlignment(.center)
                            .padding(.vertical, 8)
                            .background(
                                RoundedRectangle(cornerRadius: 10, style: .continuous)
                                    .fill(Color.nkInputBackground)
                            )

                        TextField("状态（最多4字）", text: $statusText)
                            .padding(.horizontal, 10)
                            .padding(.vertical, 8)
                            .background(
                                RoundedRectangle(cornerRadius: 10, style: .continuous)
                                    .fill(Color.nkInputBackground)
                            )
                    }
                }
                .padding(14)
                .background(
                    RoundedRectangle(cornerRadius: 14, style: .continuous)
                        .fill(Color.nkCardBackground)
                )

                Button {
                    let text = String(statusText.trimmingCharacters(in: .whitespacesAndNewlines).prefix(4))
                    let emoji = selectedEmoji.trimmingCharacters(in: .whitespacesAndNewlines)
                    onSave(emoji.isEmpty ? "❔" : String(emoji.prefix(2)), text.isEmpty ? "未知" : text)
                    dismiss()
                } label: {
                    Text("使用该状态")
                        .font(.headline)
                        .foregroundStyle(Color.nkTextOnAccent)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 12)
                        .background(Color.nkAccent)
                        .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
                }
            }
            .padding(16)
        }
        .background(Color.nkBackground.ignoresSafeArea())
        .navigationTitle("设置状态")
        .navigationBarTitleDisplayMode(.inline)
    }
}

@MainActor
private struct SettingsView: View {
    let onLogout: () -> Void

    var body: some View {
        List {
            Section("通用") {
                settingRow(icon: "bell.badge.fill", title: "通知设置")
                settingRow(icon: "paintpalette.fill", title: "主题偏好")
                settingRow(icon: "lock.shield.fill", title: "隐私与安全")
                settingRow(icon: "info.circle.fill", title: "关于纽扣")
            }

            Section {
                Button(role: .destructive) {
                    onLogout()
                } label: {
                    Label("退出登录", systemImage: "rectangle.portrait.and.arrow.right")
                }
            }
        }
        .scrollContentBackground(.hidden)
        .background(Color.nkBackground)
        .navigationTitle("设置")
    }

    private func settingRow(icon: String, title: String) -> some View {
        HStack(spacing: 10) {
            Image(systemName: icon)
                .foregroundStyle(Color.nkAccent)
            Text(title)
                .foregroundStyle(Color.nkPrimaryText)
        }
    }
}
