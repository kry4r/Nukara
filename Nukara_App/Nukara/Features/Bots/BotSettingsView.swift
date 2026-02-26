import SwiftUI

@MainActor
struct BotSettingsView: View {
    @Environment(\.dismiss) private var dismiss
    @Environment(\.appEnvironment) private var appEnvironment

    let botID: String

    @State private var bot: Bot?
    @State private var isLoading = false
    @State private var isSaving = false
    @State private var errorMessage: String?

    @State private var selectedGender: BotGender = .unknown
    @State private var speakingStyleInput = ""
    @State private var backgroundInput = ""
    @State private var traitInput = ""

    @State private var speakingStyleAdds: [String] = []
    @State private var backgroundAdds: [String] = []
    @State private var traitAdds: [String] = []

    var body: some View {
        NavigationStack {
            Group {
                if isLoading, bot == nil {
                    ProgressView()
                        .frame(maxWidth: .infinity, maxHeight: .infinity)
                } else if let bot {
                    ScrollView {
                        VStack(spacing: 14) {
                            headerCard(bot: bot)

                            editableCard(
                                title: "说话风格",
                                baseValues: splitSegments(bot.speakingStyle),
                                addedValues: speakingStyleAdds,
                                input: $speakingStyleInput,
                                placeholder: "新增风格，如：温柔直白"
                            ) {
                                appendItem(from: speakingStyleInput, into: &speakingStyleAdds, baseValues: splitSegments(bot.speakingStyle))
                                speakingStyleInput = ""
                            }

                            editableCard(
                                title: "背景故事",
                                baseValues: splitSegments(bot.background),
                                addedValues: backgroundAdds,
                                input: $backgroundInput,
                                placeholder: "新增设定，如：旅行摄影师"
                            ) {
                                appendItem(from: backgroundInput, into: &backgroundAdds, baseValues: splitSegments(bot.background))
                                backgroundInput = ""
                            }

                            editableCard(
                                title: "标签",
                                baseValues: bot.traits,
                                addedValues: traitAdds,
                                input: $traitInput,
                                placeholder: "新增标签，如：治愈"
                            ) {
                                appendItem(from: traitInput, into: &traitAdds, baseValues: bot.traits)
                                traitInput = ""
                            }

                            genderCard

                            if let errorMessage {
                                Text(errorMessage)
                                    .font(.subheadline)
                                    .foregroundStyle(.red)
                                    .frame(maxWidth: .infinity, alignment: .leading)
                                    .padding(.horizontal, 16)
                            }
                        }
                        .padding(.vertical, 12)
                    }
                } else {
                    Text("未找到角色")
                        .foregroundStyle(Color.nkPrimaryText)
                        .frame(maxWidth: .infinity, maxHeight: .infinity)
                }
            }
            .background(Color.nkBackground.ignoresSafeArea())
            .navigationTitle("人物设定")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button("关闭") {
                        dismiss()
                    }
                    .foregroundStyle(Color.nkPrimaryText)
                }

                ToolbarItem(placement: .topBarTrailing) {
                    Button(isSaving ? "保存中..." : "保存") {
                        Task { await saveChanges() }
                    }
                    .disabled(isSaving || !hasChanges)
                    .foregroundStyle(Color.nkPrimaryText)
                }
            }
            .task {
                await loadBot()
            }
        }
    }

    private var hasChanges: Bool {
        guard let bot else { return false }
        return !speakingStyleAdds.isEmpty || !backgroundAdds.isEmpty || !traitAdds.isEmpty || selectedGender != bot.gender
    }

    private var genderCard: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("性别")
                .font(.headline)
                .foregroundStyle(Color.nkPrimaryText)

            Picker("性别", selection: $selectedGender) {
                ForEach(BotGender.allCases) { option in
                    Text(option.displayName).tag(option)
                }
            }
            .pickerStyle(.segmented)
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            RoundedRectangle(cornerRadius: 16, style: .continuous)
                .fill(Color.nkCardBackground)
        )
        .padding(.horizontal, 16)
    }

    private func headerCard(bot: Bot) -> some View {
        VStack(spacing: 10) {
            AvatarView(name: bot.name, imageData: bot.avatarData, size: 86)
            Text(bot.name)
                .font(.title3.weight(.semibold))
                .foregroundStyle(Color.nkPrimaryText)
            Text(bot.summary)
                .font(.subheadline)
                .foregroundStyle(Color.nkTextSecondary)
                .multilineTextAlignment(.center)
        }
        .frame(maxWidth: .infinity)
        .padding(16)
        .background(
            RoundedRectangle(cornerRadius: 16, style: .continuous)
                .fill(Color.nkCardBackground)
        )
        .padding(.horizontal, 16)
    }

    private func editableCard(
        title: String,
        baseValues: [String],
        addedValues: [String],
        input: Binding<String>,
        placeholder: String,
        onAdd: @escaping () -> Void
    ) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(title)
                .font(.headline)
                .foregroundStyle(Color.nkPrimaryText)

            if !baseValues.isEmpty {
                Text("已有")
                    .font(.caption)
                    .foregroundStyle(Color.nkTextSecondary)
                tagsWrap(values: baseValues, color: Color.nkAccent.opacity(0.12))
            }

            if !addedValues.isEmpty {
                Text("新增")
                    .font(.caption)
                    .foregroundStyle(Color.nkTextSecondary)
                tagsWrap(values: addedValues, color: Color.nkAccent.opacity(0.3))
            }

            HStack(spacing: 8) {
                TextField(placeholder, text: input)
                    .padding(.horizontal, 10)
                    .padding(.vertical, 8)
                    .background(
                        RoundedRectangle(cornerRadius: 10, style: .continuous)
                            .fill(Color.nkInputBackground)
                    )

                Button("添加") {
                    onAdd()
                }
                .font(.subheadline.weight(.semibold))
                .foregroundStyle(Color.nkTextOnAccent)
                .padding(.horizontal, 10)
                .padding(.vertical, 8)
                .background(Color.nkAccent)
                .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
            }
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            RoundedRectangle(cornerRadius: 16, style: .continuous)
                .fill(Color.nkCardBackground)
        )
        .padding(.horizontal, 16)
    }

    private func tagsWrap(values: [String], color: Color) -> some View {
        FlexibleTagView(values: values, color: color)
    }

    private func loadBot() async {
        isLoading = true
        defer { isLoading = false }

        do {
            let loaded = try await appEnvironment.botRepository.getBot(botID: botID)
            bot = loaded
            selectedGender = loaded.gender
            speakingStyleAdds = []
            backgroundAdds = []
            traitAdds = []
            errorMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func saveChanges() async {
        guard let bot else { return }

        isSaving = true
        defer { isSaving = false }

        do {
            let updated = try await appEnvironment.botRepository.appendPersona(
                botID: botID,
                input: AppendBotPersonaInput(
                    speakingStyleAdds: speakingStyleAdds,
                    backgroundAdds: backgroundAdds,
                    traitAdds: traitAdds,
                    gender: selectedGender == bot.gender ? nil : selectedGender
                )
            )

            self.bot = updated
            speakingStyleAdds = []
            backgroundAdds = []
            traitAdds = []
            errorMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func splitSegments(_ value: String) -> [String] {
        value
            .split(separator: "|")
            .map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }
            .filter { !$0.isEmpty }
    }

    private func appendItem(from raw: String, into target: inout [String], baseValues: [String]) {
        let value = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !value.isEmpty else { return }
        guard !baseValues.contains(value), !target.contains(value) else { return }
        target.append(value)
    }
}

private struct FlexibleTagView: View {
    let values: [String]
    let color: Color

    var body: some View {
        LazyVGrid(columns: [GridItem(.adaptive(minimum: 72), spacing: 8, alignment: .leading)], alignment: .leading, spacing: 8) {
            ForEach(values, id: \.self) { value in
                Text(value)
                    .font(.caption)
                    .foregroundStyle(Color.nkPrimaryText)
                    .lineLimit(1)
                    .padding(.horizontal, 10)
                    .padding(.vertical, 6)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(color)
                    .clipShape(Capsule())
            }
        }
    }
}
