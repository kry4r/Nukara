import PhotosUI
import SwiftUI

@MainActor
struct BotListView: View {
    @Environment(\.appEnvironment) private var appEnvironment

    @State private var bots: [Bot] = []
    @State private var isLoading = false
    @State private var errorMessage: String?
    @State private var isCreatePresented = false
    @State private var searchQuery = ""

    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                topBar

                if isLoading && bots.isEmpty {
                    Spacer()
                    ProgressView()
                    Spacer()
                } else {
                    ScrollView {
                        LazyVStack(spacing: 12) {
                            ForEach(filteredBots) { bot in
                                NavigationLink {
                                    BotSettingsView(botID: bot.id)
                                } label: {
                                    BotCard(bot: bot)
                                }
                                .buttonStyle(.plain)
                            }
                        }
                        .padding(16)
                    }
                    .refreshable {
                        await loadBots()
                    }
                }
            }
            .background(Color.nkBackground.ignoresSafeArea())
            .task {
                if bots.isEmpty { await loadBots() }
            }
            .alert("加载失败", isPresented: Binding(get: {
                errorMessage != nil
            }, set: { value in
                if !value { errorMessage = nil }
            })) {
                Button("确定", role: .cancel) {}
            } message: {
                Text(errorMessage ?? "未知错误")
            }
            .sheet(isPresented: $isCreatePresented) {
                BotCreationView {
                    await loadBots()
                }
            }
        }
    }

    private var topBar: some View {
        VStack(spacing: 10) {
            HStack {
                Spacer()
                Text("通讯录")
                    .font(.title3.weight(.semibold))
                    .foregroundStyle(Color.nkPrimaryText)
                Spacer()
                Button {
                    isCreatePresented = true
                } label: {
                    Image(systemName: "plus")
                        .font(.system(size: 17, weight: .bold))
                        .foregroundStyle(Color.nkPrimaryText)
                        .frame(width: 32, height: 32)
                        .background(Color.nkCardBackground)
                        .clipShape(Circle())
                }
            }

            HStack(spacing: 8) {
                Image(systemName: "magnifyingglass")
                    .foregroundStyle(Color.nkTextMuted)
                TextField("搜索角色 / 标签 / 简介", text: $searchQuery)
                    .font(.subheadline)
                    .foregroundStyle(Color.nkPrimaryText)
            }
            .padding(.horizontal, 12)
            .padding(.vertical, 9)
            .background(
                RoundedRectangle(cornerRadius: 12, style: .continuous)
                    .fill(Color.nkCardBackground)
            )
        }
        .padding(.horizontal, 16)
        .padding(.top, 8)
        .padding(.bottom, 10)
    }

    private func loadBots() async {
        isLoading = true
        defer { isLoading = false }

        do {
            bots = try await appEnvironment.botRepository.listBots()
            errorMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private var filteredBots: [Bot] {
        let query = searchQuery.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !query.isEmpty else { return bots }

        return bots.filter { bot in
            bot.name.localizedCaseInsensitiveContains(query)
            || bot.summary.localizedCaseInsensitiveContains(query)
            || bot.traits.joined(separator: " ").localizedCaseInsensitiveContains(query)
        }
    }
}

private struct BotCard: View {
    let bot: Bot

    var body: some View {
        HStack(spacing: 12) {
            AvatarView(name: bot.name, imageData: bot.avatarData, size: 52)

            VStack(alignment: .leading, spacing: 6) {
                Text(bot.name)
                    .font(.headline)
                    .foregroundStyle(Color.nkPrimaryText)
                Text(bot.summary)
                    .font(.subheadline)
                    .foregroundStyle(Color.nkTextSecondary)
                    .lineLimit(1)
                HStack(spacing: 6) {
                    Text(bot.gender.displayName)
                    Text("·")
                    Text(bot.traits.isEmpty ? "未设置标签" : bot.traits.joined(separator: " · "))
                }
                .font(.caption)
                .foregroundStyle(Color.nkTextMuted)
                .lineLimit(1)
            }

            Spacer()

            Image(systemName: "chevron.right")
                .font(.caption.weight(.semibold))
                .foregroundStyle(Color.nkTextMuted)
        }
        .padding(14)
        .background(
            RoundedRectangle(cornerRadius: 16, style: .continuous)
                .fill(Color.nkCardBackground)
        )
    }
}

@MainActor
private struct BotCreationView: View {
    @Environment(\.dismiss) private var dismiss
    @Environment(\.appEnvironment) private var appEnvironment

    @State private var name = ""
    @State private var summary = ""
    @State private var speakingStyle = ""
    @State private var background = ""
    @State private var traits = ""
    @State private var gender: BotGender = .unknown
    @State private var pickedAvatarItem: PhotosPickerItem?
    @State private var avatarData: Data?
    @State private var isSubmitting = false
    @State private var errorMessage: String?

    let onCreated: () async -> Void

    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                ScrollView {
                    VStack(spacing: 16) {
                        VStack(spacing: 12) {
                            AvatarView(name: name.isEmpty ? "新" : name, imageData: avatarData, size: 92)
                            PhotosPicker(selection: $pickedAvatarItem, matching: .images) {
                                Text("上传头像")
                                    .font(.subheadline.weight(.semibold))
                                    .foregroundStyle(Color.nkPrimaryText)
                                    .padding(.horizontal, 14)
                                    .padding(.vertical, 8)
                                    .background(Color.nkAccent.opacity(0.55))
                                    .clipShape(Capsule())
                            }
                        }
                        .frame(maxWidth: .infinity)
                        .padding(16)
                        .background(
                            RoundedRectangle(cornerRadius: 18, style: .continuous)
                                .fill(Color.nkCardBackground)
                        )

                        formCard(title: "基础", content: {
                            formRow(title: "名字", placeholder: "给角色起个名字", text: $name)
                            formRow(title: "简介", placeholder: "一句话介绍角色", text: $summary)
                        })

                        formCard(title: "角色设定", content: {
                            HStack {
                                Text("性别：")
                                    .font(.subheadline)
                                    .foregroundStyle(Color.nkTextSecondary)
                                    .frame(width: 74, alignment: .leading)
                                Picker("性别", selection: $gender) {
                                    ForEach(BotGender.allCases) { option in
                                        Text(option.displayName).tag(option)
                                    }
                                }
                                .pickerStyle(.menu)
                            }

                            formRow(title: "风格", placeholder: "说话风格", text: $speakingStyle)
                            labeledMultilineRow(title: "背景", placeholder: "背景故事", text: $background)
                            formRow(title: "标签", placeholder: "性格标签，用逗号分隔", text: $traits)
                        })

                        if let errorMessage {
                            Text(errorMessage)
                                .font(.subheadline)
                                .foregroundStyle(.red)
                                .frame(maxWidth: .infinity, alignment: .leading)
                        }
                    }
                    .padding(16)
                }

                Button {
                    Task { await submit() }
                } label: {
                    Text(isSubmitting ? "创建中..." : "创建角色")
                        .font(.headline)
                        .foregroundStyle(Color.nkTextOnAccent)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 14)
                        .background(Color.nkAccent)
                        .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
                }
                .disabled(isSubmitting || name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                .padding(16)
                .background(Color.nkBackground)
            }
            .background(Color.nkBackground.ignoresSafeArea())
            .navigationTitle("创建角色")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button("取消") { dismiss() }
                        .foregroundStyle(Color.nkPrimaryText)
                }
            }
            .onChange(of: pickedAvatarItem) {
                Task {
                    guard let item = pickedAvatarItem,
                          let data = try? await item.loadTransferable(type: Data.self) else {
                        return
                    }
                    avatarData = data
                    pickedAvatarItem = nil
                }
            }
        }
    }

    private func formCard<Content: View>(title: String, @ViewBuilder content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(title)
                .font(.headline)
                .foregroundStyle(Color.nkPrimaryText)

            content()
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .fill(Color.nkCardBackground)
        )
    }

    private func formRow(title: String, placeholder: String, text: Binding<String>) -> some View {
        HStack(spacing: 10) {
            Text("\(title)：")
                .font(.subheadline)
                .foregroundStyle(Color.nkTextSecondary)
                .frame(width: 74, alignment: .leading)

            TextField(placeholder, text: text)
                .padding(.horizontal, 10)
                .padding(.vertical, 8)
                .background(
                    RoundedRectangle(cornerRadius: 10, style: .continuous)
                        .fill(Color.nkInputBackground)
                )
        }
    }

    private func labeledMultilineRow(title: String, placeholder: String, text: Binding<String>) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("\(title)：")
                .font(.subheadline)
                .foregroundStyle(Color.nkTextSecondary)

            TextField(placeholder, text: text, axis: .vertical)
                .lineLimit(3 ... 5)
                .padding(.horizontal, 10)
                .padding(.vertical, 8)
                .background(
                    RoundedRectangle(cornerRadius: 10, style: .continuous)
                        .fill(Color.nkInputBackground)
                )
        }
    }

    private func submit() async {
        isSubmitting = true
        defer { isSubmitting = false }

        let input = CreateBotInput(
            name: name,
            summary: summary,
            speakingStyle: speakingStyle,
            background: background,
            traits: traits
                .split(separator: ",")
                .map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }
                .filter { !$0.isEmpty },
            gender: gender,
            avatarData: avatarData
        )

        do {
            _ = try await appEnvironment.botRepository.createBot(input)
            await onCreated()
            dismiss()
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}
