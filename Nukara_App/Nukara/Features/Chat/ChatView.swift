import CoreLocation
import MapKit
import PhotosUI
import SwiftUI

@MainActor
struct ChatScene: View {
    @StateObject private var store: ChatStore
    @State private var isSettingsPresented = false
    @State private var isBackgroundPickerPresented = false

    init(
        conversation: Conversation,
        conversationRepository: ConversationRepositoryProtocol,
        chatRepository: ChatRepositoryProtocol
    ) {
        _store = StateObject(
            wrappedValue: ChatStore(
                conversation: conversation,
                conversationRepository: conversationRepository,
                chatRepository: chatRepository
            )
        )
    }

    var body: some View {
        ChatView(store: store)
            .navigationTitle(store.state.botName)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Menu {
                        Button {
                            isSettingsPresented = true
                        } label: {
                            Label("人物设定", systemImage: "person.text.rectangle")
                        }

                        Button {
                            isBackgroundPickerPresented = true
                        } label: {
                            Label("更换背景", systemImage: "photo")
                        }
                    } label: {
                        Image(systemName: "ellipsis")
                            .foregroundStyle(Color.nkPrimaryText)
                    }
                }
            }
            .task {
                store.send(.onAppear)
            }
            .sheet(isPresented: $isSettingsPresented, onDismiss: {
                store.refreshPreferences()
            }) {
                BotSettingsView(botID: store.state.botID)
            }
            .sheet(isPresented: $isBackgroundPickerPresented, onDismiss: {
                store.refreshPreferences()
            }) {
                ChatBackgroundPickerView(botID: store.state.botID)
            }
    }
}

@MainActor
private struct ChatView: View {
    @ObservedObject var store: ChatStore

    @State private var showAttachmentBubble = false
    @State private var isPhotoPickerPresented = false
    @State private var pickedPhotoItem: PhotosPickerItem?
    @State private var isLocationPickerPresented = false
    @State private var previewImage: UIImage?
    @State private var previewLocation: ChatLocation?

    var body: some View {
        VStack(spacing: 0) {
            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(spacing: 12) {
                        ForEach(store.state.messages) { message in
                            MessageRow(
                                message: message,
                                botName: store.state.botName,
                                botAvatarData: store.state.botAvatarData,
                                onImageTapped: { image in
                                    previewImage = image
                                },
                                onLocationTapped: { location in
                                    previewLocation = location
                                },
                                onQuote: { msg in
                                    store.send(.quoteTapped(msg))
                                }
                            )
                            .id(message.id)
                        }

                        if store.state.isRemoteTyping {
                            HStack(spacing: 8) {
                                AvatarView(name: store.state.botName, imageData: store.state.botAvatarData, size: 28)
                                Text("对方正在输入...")
                                    .font(.caption)
                                    .foregroundStyle(Color.nkTextSecondary)
                                Spacer()
                            }
                        }
                    }
                    .padding(.horizontal, 12)
                    .padding(.top, 12)
                    .padding(.bottom, 10)
                }
                .onChange(of: store.state.messages.count) {
                    if let lastID = store.state.messages.last?.id {
                        withAnimation(.easeOut(duration: 0.2)) {
                            proxy.scrollTo(lastID, anchor: .bottom)
                        }
                    }
                }
            }

            chatInputBar
                .padding(.horizontal, 10)
                .padding(.top, 8)
                .padding(.bottom, 12)
                .background(
                    RoundedRectangle(cornerRadius: 20, style: .continuous)
                        .fill(Color.nkCardBackground)
                        .ignoresSafeArea(edges: .bottom)
                )
        }
        .background(backgroundView.ignoresSafeArea())
        .photosPicker(
            isPresented: $isPhotoPickerPresented,
            selection: $pickedPhotoItem,
            matching: .images
        )
        .onChange(of: pickedPhotoItem) {
            Task {
                guard let item = pickedPhotoItem,
                      let data = try? await item.loadTransferable(type: Data.self) else {
                    return
                }
                store.send(.imagePicked(data))
                pickedPhotoItem = nil
            }
        }
        .sheet(isPresented: $isLocationPickerPresented) {
            LocationPickerView { location in
                store.send(.locationPicked(location))
            }
        }
        .fullScreenCover(isPresented: Binding(get: {
            previewImage != nil
        }, set: { value in
            if !value { previewImage = nil }
        })) {
            if let previewImage {
                ImagePreviewView(image: previewImage)
            }
        }
        .sheet(isPresented: Binding(get: {
            previewLocation != nil
        }, set: { value in
            if !value { previewLocation = nil }
        })) {
            if let previewLocation {
                LocationPreviewView(location: previewLocation)
            }
        }
        .onTapGesture {
            if showAttachmentBubble {
                withAnimation(.spring(response: 0.2, dampingFraction: 0.86)) {
                    showAttachmentBubble = false
                }
            }
        }
        .alert("提示", isPresented: Binding(get: {
            store.state.errorMessage != nil
        }, set: { value in
            if !value { store.clearError() }
        })) {
            Button("确定", role: .cancel) {}
        } message: {
            Text(store.state.errorMessage ?? "")
        }
    }

    private var chatInputBar: some View {
        VStack(spacing: 6) {
            if let quoted = store.state.quotedMessage {
                HStack(spacing: 8) {
                    Rectangle()
                        .fill(Color.nkAccent)
                        .frame(width: 3)
                    VStack(alignment: .leading, spacing: 2) {
                        Text(quoted.sender == .bot ? store.state.botName : "我")
                            .font(.caption.weight(.semibold))
                            .foregroundStyle(Color.nkTextSecondary)
                        Text(quoted.text)
                            .font(.caption)
                            .foregroundStyle(Color.nkTextMuted)
                            .lineLimit(2)
                    }
                    Spacer()
                    Button {
                        store.send(.clearQuote)
                    } label: {
                        Image(systemName: "xmark.circle.fill")
                            .foregroundStyle(Color.nkTextMuted)
                    }
                }
                .padding(.horizontal, 12)
                .padding(.vertical, 6)
                .background(Color.nkAccent.opacity(0.1))
                .clipShape(RoundedRectangle(cornerRadius: 8, style: .continuous))
            }

            HStack(spacing: 10) {
            Button {
                withAnimation(.spring(response: 0.2, dampingFraction: 0.86)) {
                    showAttachmentBubble.toggle()
                }
            } label: {
                Image(systemName: "plus")
                    .font(.system(size: 18, weight: .semibold))
                    .foregroundStyle(Color.nkPrimaryText)
                    .frame(width: 34, height: 34)
                    .background(Color.nkAccent.opacity(0.12))
                    .clipShape(Circle())
            }

            TextField("输入消息...", text: Binding(
                get: { store.state.inputText },
                set: { store.send(.inputChanged($0)) }
            ), axis: .vertical)
            .lineLimit(1 ... 4)
            .padding(.horizontal, 12)
            .padding(.vertical, 10)
            .background(
                RoundedRectangle(cornerRadius: 18, style: .continuous)
                    .fill(Color.nkInputBackground)
            )

            Button {
                store.send(.sendTapped)
            } label: {
                Image(systemName: "arrow.up")
                    .font(.system(size: 16, weight: .bold))
                    .foregroundStyle(Color.nkTextOnAccent)
                    .frame(width: 34, height: 34)
                    .background(Color.nkAccent)
                    .clipShape(Circle())
            }
            .disabled(store.state.inputText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
        }
        .overlay(alignment: .topLeading) {
            if showAttachmentBubble {
                AttachmentBubbleMenu(
                    onSendLocation: {
                        isLocationPickerPresented = true
                        withAnimation(.spring(response: 0.2, dampingFraction: 0.86)) {
                            showAttachmentBubble = false
                        }
                    },
                    onSendImage: {
                        isPhotoPickerPresented = true
                        withAnimation(.spring(response: 0.2, dampingFraction: 0.86)) {
                            showAttachmentBubble = false
                        }
                    }
                )
                .offset(x: 0, y: -102)
                .transition(.move(edge: .bottom).combined(with: .opacity))
            }
        }
        }
    }

    @ViewBuilder
    private var backgroundView: some View {
        if let imageData = store.state.chatBackgroundImageData,
           let image = UIImage(data: imageData) {
            Image(uiImage: image)
                .resizable()
                .scaledToFill()
                .overlay(Color.nkBackground.opacity(0.15))
        } else if let hex = store.state.chatBackgroundColorHex {
            Color(hex: hex)
        } else {
            switch store.state.chatBackgroundStyle {
            case .lightPaper:
                Color.nkBackground
            case .softMint:
                LinearGradient(colors: [Color.nkBackground, Color.nkAccent.opacity(0.18)], startPoint: .top, endPoint: .bottom)
            case .peachGlow:
                LinearGradient(colors: [Color.nkBackground, Color.nkSecondaryAccent.opacity(0.25)], startPoint: .top, endPoint: .bottom)
            case .calmNight:
                LinearGradient(colors: [Color.nkWarmBackground, Color.nkAccentDark.opacity(0.2)], startPoint: .top, endPoint: .bottom)
            }
        }
    }
}

private struct MessageRow: View {
    let message: ChatMessage
    let botName: String
    let botAvatarData: Data?
    let onImageTapped: (UIImage) -> Void
    let onLocationTapped: (ChatLocation) -> Void
    let onQuote: (ChatMessage) -> Void

    var body: some View {
        HStack(alignment: .bottom, spacing: 8) {
            if message.sender == .bot {
                AvatarView(name: botName, imageData: botAvatarData, size: 32)
                bubble
                Spacer(minLength: 30)
            } else {
                Spacer(minLength: 30)
                bubble
                AvatarView(name: "我", imageData: nil, size: 32)
            }
        }
    }

    private var bubble: some View {
        MessageBubble(
            message: message,
            onImageTapped: onImageTapped,
            onLocationTapped: onLocationTapped,
            onQuote: onQuote
        )
    }
}

private struct MessageBubble: View {
    let message: ChatMessage
    let onImageTapped: (UIImage) -> Void
    let onLocationTapped: (ChatLocation) -> Void
    let onQuote: (ChatMessage) -> Void

    @State private var animateProactive = false
    @State private var proactiveAnimationEnabled = false
    @State private var appearScale: CGFloat = 0.92
    @State private var appearOpacity: Double = 0.0

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            if let imageData = message.imageData,
               let uiImage = UIImage(data: imageData) {
                Image(uiImage: uiImage)
                    .resizable()
                    .scaledToFill()
                    .frame(width: 170, height: 170)
                    .clipped()
                    .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
                    .onTapGesture {
                        onImageTapped(uiImage)
                    }
            }

            if let location = message.location {
                LocationBubbleCard(location: location) {
                    onLocationTapped(location)
                }
            }

            if !message.text.isEmpty {
                Text(AttributedString.chatFormatted(message.text))
                    .font(.body)
                    .foregroundStyle(Color.nkPrimaryText)
            } else if message.status == .streaming {
                TypingDotsView()
            }
        }
        .padding(10)
        .background(backgroundColor)
        .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
        .overlay(alignment: .topTrailing) {
            if message.isProactive && proactiveAnimationEnabled {
                HStack(spacing: 4) {
                    Text("💬")
                    Text("✨")
                }
                .font(.caption)
                .offset(y: animateProactive ? -8 : -2)
                .opacity(animateProactive ? 1 : 0.45)
            }
        }
        .offset(x: message.isProactive && proactiveAnimationEnabled && animateProactive ? 2 : -2)
        .scaleEffect(appearScale)
        .opacity(appearOpacity)
        .onAppear {
            withAnimation(.spring(response: 0.22, dampingFraction: 0.78)) {
                appearScale = 1
                appearOpacity = 1
            }

            guard message.isProactive else { return }
            proactiveAnimationEnabled = true
            withAnimation(.easeInOut(duration: 0.55).repeatForever(autoreverses: true)) {
                animateProactive = true
            }

            Task {
                try? await Task.sleep(for: .seconds(7))
                guard !Task.isCancelled else { return }
                withAnimation(.easeOut(duration: 0.25)) {
                    proactiveAnimationEnabled = false
                    animateProactive = false
                }
            }
        }
        .contextMenu {
            Button {
                UIPasteboard.general.string = message.text
            } label: {
                Label("复制", systemImage: "doc.on.doc")
            }

            Button {
                onQuote(message)
            } label: {
                Label("引用", systemImage: "quote.bubble")
            }
        }
    }

    private var backgroundColor: Color {
        if message.isProactive {
            return Color.nkAccentSecondary
        }

        if message.sender == .user {
            return Color.nkBubbleUser.opacity(0.85)
        }

        return Color.nkBubbleBot
    }
}

private struct TypingDotsView: View {
    @State private var phase = 0

    var body: some View {
        HStack(spacing: 4) {
            ForEach(0..<3) { index in
                Circle()
                    .fill(Color.nkPrimaryText.opacity(0.4))
                    .frame(width: 6, height: 6)
                    .offset(y: phase == index ? -3 : 0)
            }
        }
        .frame(height: 12)
        .onAppear {
            withAnimation(.easeInOut(duration: 0.4).repeatForever(autoreverses: true)) {
                phase = 1
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) {
                withAnimation(.easeInOut(duration: 0.4).repeatForever(autoreverses: true)) {
                    phase = 2
                }
            }
        }
    }
}

private struct AttachmentBubbleMenu: View {
    let onSendLocation: () -> Void
    let onSendImage: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            AttachmentBubbleItem(
                icon: "location.fill",
                title: "发送位置",
                action: onSendLocation
            )
            AttachmentBubbleItem(
                icon: "photo.fill.on.rectangle.fill",
                title: "发送图片",
                action: onSendImage
            )
        }
        .frame(width: 132, alignment: .leading)
        .padding(.horizontal, 10)
        .padding(.vertical, 9)
        .background(
            RoundedRectangle(cornerRadius: 22, style: .continuous)
                .fill(.ultraThinMaterial)
                .overlay(
                    RoundedRectangle(cornerRadius: 22, style: .continuous)
                        .stroke(Color.white.opacity(0.42), lineWidth: 0.8)
                )
                .shadow(color: Color.black.opacity(0.14), radius: 12, y: 4)
        )
        .overlay(alignment: .bottomLeading) {
            Triangle()
                .fill(Color.white.opacity(0.68))
                .frame(width: 14, height: 8)
                .offset(x: 14, y: 8)
        }
    }
}

private struct AttachmentBubbleItem: View {
    let icon: String
    let title: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: 10) {
                Image(systemName: icon)
                    .font(.system(size: 14, weight: .semibold))
                    .frame(width: 18, alignment: .center)
                    .foregroundStyle(Color.nkPrimaryText)

                Text(title)
                    .font(.subheadline)
                    .foregroundStyle(Color.nkPrimaryText)
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, 10)
            .padding(.vertical, 8)
            .background(Color.nkWarmBackground)
            .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
        }
        .buttonStyle(.plain)
    }
}

private struct Triangle: Shape {
    func path(in rect: CGRect) -> Path {
        var path = Path()
        path.move(to: CGPoint(x: rect.minX, y: rect.minY))
        path.addLine(to: CGPoint(x: rect.maxX, y: rect.minY))
        path.addLine(to: CGPoint(x: rect.midX, y: rect.maxY))
        path.closeSubpath()
        return path
    }
}

private struct LocationBubbleCard: View {
    let location: ChatLocation
    let onTap: () -> Void

    // Total card width (including paddings) stays fixed to avoid overflowing chat bubble.
    private let cardWidth: CGFloat = 180
    private let contentInset: CGFloat = 8

    var body: some View {
        Button(action: onTap) {
            VStack(alignment: .leading, spacing: 8) {
                StaticMapThumbnailView(location: location)
                    .frame(height: 92)
                    .frame(maxWidth: .infinity)
                    .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))

                HStack(spacing: 6) {
                    Image(systemName: "location.fill")
                        .font(.caption)
                        .foregroundStyle(Color.nkPrimaryText)
                    Text(location.name)
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(Color.nkPrimaryText)
                        .lineLimit(1)
                }

                Text(String(format: "%.5f, %.5f", location.latitude, location.longitude))
                    .font(.caption2)
                    .foregroundStyle(Color.nkTextMuted)
            }
            .padding(contentInset)
            .frame(width: cardWidth, alignment: .leading)
            .background(
                RoundedRectangle(cornerRadius: 12, style: .continuous)
                    .fill(Color.nkCardBackground)
            )
            .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
        }
        .buttonStyle(.plain)
    }
}

@MainActor
private struct StaticMapThumbnailView: View {
    let location: ChatLocation
    @State private var image: UIImage?

    var body: some View {
        ZStack {
            if let image {
                Image(uiImage: image)
                    .resizable()
                    .scaledToFill()
            } else {
                LinearGradient(
                    colors: [Color.nkBackground, Color.nkAccent.opacity(0.3)],
                    startPoint: .top,
                    endPoint: .bottom
                )
                Image(systemName: "map.fill")
                    .font(.title3)
                    .foregroundStyle(Color.nkTextSecondary)
            }
        }
        .task(id: location.latitude.description + location.longitude.description) {
            await generateSnapshot()
        }
    }

    private func generateSnapshot() async {
        let options = MKMapSnapshotter.Options()
        options.region = MKCoordinateRegion(
            center: CLLocationCoordinate2D(latitude: location.latitude, longitude: location.longitude),
            span: MKCoordinateSpan(latitudeDelta: 0.012, longitudeDelta: 0.012)
        )
        options.size = CGSize(width: 440, height: 220)
        options.scale = UIScreen.main.scale

        let snapshotter = MKMapSnapshotter(options: options)
        do {
            let snapshot = try await snapshotter.start()
            image = snapshot.image
        } catch {
            image = nil
        }
    }
}

@MainActor
private struct ChatBackgroundPickerView: View {
    @Environment(\.dismiss) private var dismiss

    @State private var selectedStyle: ChatBackgroundStyle = .lightPaper
    @State private var selectedColorHex: String?
    @State private var selectedImageData: Data?
    @State private var imagePickerItem: PhotosPickerItem?

    let botID: String

    private let colorChoices = ["#F5F8EF", "#7BA05B", "#C4A882", "#F0F5E8", "#FFFFFF"]

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    Text("颜色")
                        .font(.headline)
                        .foregroundStyle(Color.nkPrimaryText)

                    HStack(spacing: 12) {
                        ForEach(colorChoices, id: \.self) { hex in
                            Button {
                                selectedColorHex = hex
                                selectedImageData = nil
                            } label: {
                                Circle()
                                    .fill(Color(hex: hex))
                                    .frame(width: 30, height: 30)
                                    .overlay {
                                        if selectedColorHex == hex {
                                            Image(systemName: "checkmark")
                                                .font(.caption.weight(.bold))
                                                .foregroundStyle(Color.nkPrimaryText)
                                        }
                                    }
                            }
                        }
                    }

                    Divider()

                    Text("渐变风格")
                        .font(.headline)
                        .foregroundStyle(Color.nkPrimaryText)

                    ForEach(ChatBackgroundStyle.allCases) { style in
                        Button {
                            selectedStyle = style
                            selectedColorHex = nil
                            selectedImageData = nil
                        } label: {
                            HStack {
                                Circle()
                                    .fill(backgroundPreview(style))
                                    .frame(width: 16, height: 16)
                                Text(style.displayName)
                                    .foregroundStyle(Color.nkPrimaryText)
                                Spacer()
                                if selectedStyle == style && selectedColorHex == nil && selectedImageData == nil {
                                    Image(systemName: "checkmark.circle.fill")
                                        .foregroundStyle(Color.nkAccent)
                                }
                            }
                            .padding(.vertical, 4)
                        }
                    }

                    Divider()

                    PhotosPicker(selection: $imagePickerItem, matching: .images) {
                        Label("上传背景图片", systemImage: "photo")
                            .font(.subheadline.weight(.semibold))
                            .foregroundStyle(Color.nkPrimaryText)
                            .padding(.horizontal, 12)
                            .padding(.vertical, 8)
                            .background(Color.nkAccent.opacity(0.5))
                            .clipShape(Capsule())
                    }

                    if selectedImageData != nil {
                        Button("清除背景图") {
                            selectedImageData = nil
                        }
                        .font(.subheadline)
                        .foregroundStyle(Color.nkTextSecondary)
                    }
                }
                .padding(16)
            }
            .background(Color.nkBackground.ignoresSafeArea())
            .navigationTitle("聊天背景")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button("取消") {
                        dismiss()
                    }
                    .foregroundStyle(Color.nkPrimaryText)
                }

                ToolbarItem(placement: .topBarTrailing) {
                    Button("完成") {
                        var preference = BotPreferencesStore.shared.preference(for: botID)
                        preference.chatBackgroundStyle = selectedStyle
                        preference.chatBackgroundColorHex = selectedColorHex
                        preference.chatBackgroundImageData = selectedImageData
                        BotPreferencesStore.shared.updatePreference(preference, for: botID)
                        dismiss()
                    }
                    .foregroundStyle(Color.nkPrimaryText)
                }
            }
            .task {
                let preference = BotPreferencesStore.shared.preference(for: botID)
                selectedStyle = preference.chatBackgroundStyle
                selectedColorHex = preference.chatBackgroundColorHex
                selectedImageData = preference.chatBackgroundImageData
            }
            .onChange(of: imagePickerItem) {
                Task {
                    guard let item = imagePickerItem,
                          let data = try? await item.loadTransferable(type: Data.self) else {
                        return
                    }
                    selectedImageData = data
                    selectedColorHex = nil
                    imagePickerItem = nil
                }
            }
        }
    }

    private func backgroundPreview(_ style: ChatBackgroundStyle) -> Color {
        switch style {
        case .lightPaper: return Color.nkBackground
        case .softMint: return Color.nkAccent.opacity(0.4)
        case .peachGlow: return Color.nkSecondaryAccent.opacity(0.6)
        case .calmNight: return Color.nkAccentDark.opacity(0.4)
        }
    }
}

@MainActor
private struct LocationPickerView: View {
    @Environment(\.dismiss) private var dismiss

    @State private var cameraPosition: MapCameraPosition = .region(
        MKCoordinateRegion(
            center: CLLocationCoordinate2D(latitude: 31.2304, longitude: 121.4737),
            span: MKCoordinateSpan(latitudeDelta: 0.08, longitudeDelta: 0.08)
        )
    )
    @State private var selectedCoordinate: CLLocationCoordinate2D?
    @State private var selectedName = ""
    @State private var isResolvingName = false

    private let applePlaceNameProvider = ApplePlaceNameProvider()
    private let amapPlaceNameProvider = AmapPlaceNameProvider(apiKey: "2a4e1d4a09cdb2a7b20bf6a6910d5e6b")

    let onPicked: (ChatLocation) -> Void

    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                MapReader { proxy in
                    Map(position: $cameraPosition) {
                        if let coordinate = selectedCoordinate {
                            Annotation("", coordinate: coordinate, anchor: .bottom) {
                                VStack(spacing: 4) {
                                    HStack(spacing: 6) {
                                        Image(systemName: "mappin.and.ellipse")
                                            .foregroundStyle(Color.nkPrimaryText)
                                        TextField("位置名称", text: $selectedName)
                                            .font(.caption)

                                        if isResolvingName {
                                            ProgressView()
                                                .controlSize(.mini)
                                        }
                                    }
                                    .padding(.horizontal, 10)
                                    .padding(.vertical, 8)
                                    .background(
                                        RoundedRectangle(cornerRadius: 12, style: .continuous)
                                            .fill(Color.white.opacity(0.95))
                                    )
                                    .shadow(color: Color.black.opacity(0.1), radius: 6, y: 2)

                                    Image(systemName: "mappin.circle.fill")
                                        .font(.title2)
                                        .foregroundStyle(Color.nkSecondaryAccent)
                                }
                            }
                        }
                    }
                    .onTapGesture { point in
                        if let coordinate = proxy.convert(point, from: .local) {
                            selectedCoordinate = coordinate
                            selectedName = String(format: "%.4f, %.4f", coordinate.latitude, coordinate.longitude)
                            Task {
                                await resolveLocationNameIfNeeded(coordinate)
                            }
                        }
                    }
                }

                VStack(spacing: 10) {
                    Text("点击地图选择位置，名称会显示在选中点上方气泡。")
                        .font(.caption)
                        .foregroundStyle(Color.nkPrimaryText.opacity(0.7))
                        .frame(maxWidth: .infinity, alignment: .leading)

                    Button {
                        guard let coordinate = selectedCoordinate else { return }
                        let name = selectedName.trimmingCharacters(in: .whitespacesAndNewlines)
                        onPicked(
                            ChatLocation(
                                latitude: coordinate.latitude,
                                longitude: coordinate.longitude,
                                name: name.isEmpty ? "我分享的位置" : name
                            )
                        )
                        dismiss()
                    } label: {
                        Text("发送位置")
                            .font(.headline)
                            .foregroundStyle(Color.nkPrimaryText)
                            .frame(maxWidth: .infinity)
                            .padding(.vertical, 12)
                            .background(Color.nkAccent)
                            .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
                    }
                    .disabled(selectedCoordinate == nil)
                }
                .padding(12)
                .background(Color.nkBackground)
            }
            .background(Color.nkBackground.ignoresSafeArea())
            .navigationTitle("选择位置")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button("取消") { dismiss() }
                        .foregroundStyle(Color.nkPrimaryText)
                }
            }
        }
    }

    private func resolveLocationNameIfNeeded(_ coordinate: CLLocationCoordinate2D) async {
        isResolvingName = true
        defer { isResolvingName = false }

        if let amapName = await amapPlaceNameProvider.reverseGeocodeName(latitude: coordinate.latitude, longitude: coordinate.longitude),
           !amapName.isEmpty {
            selectedName = amapName
            return
        }

        if let appleName = await applePlaceNameProvider.reverseGeocodeName(latitude: coordinate.latitude, longitude: coordinate.longitude),
           !appleName.isEmpty {
            selectedName = appleName
        }
    }
}

private protocol PlaceNameProvider {
    func reverseGeocodeName(latitude: Double, longitude: Double) async -> String?
}

private struct ApplePlaceNameProvider: PlaceNameProvider {
    func reverseGeocodeName(latitude: Double, longitude: Double) async -> String? {
        let geocoder = CLGeocoder()
        let location = CLLocation(latitude: latitude, longitude: longitude)
        do {
            let placemarks = try await geocoder.reverseGeocodeLocation(location)
            if let place = placemarks.first {
                return place.name ?? place.locality ?? place.subLocality
            }
        } catch {
            return nil
        }
        return nil
    }
}

private struct AmapPlaceNameProvider: PlaceNameProvider {
    let apiKey: String

    func reverseGeocodeName(latitude: Double, longitude: Double) async -> String? {
        // Reserved for future real integration.
        // Expected Amap endpoint:
        // https://restapi.amap.com/v3/geocode/regeo?key=<KEY>&location=<lng>,<lat>
        _ = apiKey
        _ = latitude
        _ = longitude
        return nil
    }
}

@MainActor
private struct ImagePreviewView: View {
    @Environment(\.dismiss) private var dismiss

    let image: UIImage

    var body: some View {
        ZStack(alignment: .topTrailing) {
            Color.black.ignoresSafeArea()

            GeometryReader { proxy in
                Image(uiImage: image)
                    .resizable()
                    .scaledToFit()
                    .frame(
                        maxWidth: max(proxy.size.width - 24, 120),
                        maxHeight: max(proxy.size.height - 24, 120)
                    )
                    .frame(width: proxy.size.width, height: proxy.size.height, alignment: .center)
            }

            Button {
                dismiss()
            } label: {
                Image(systemName: "xmark")
                    .font(.system(size: 14, weight: .bold))
                    .foregroundStyle(.white)
                    .frame(width: 30, height: 30)
                    .background(.black.opacity(0.55))
                    .clipShape(Circle())
            }
            .padding(.trailing, 16)
            .padding(.top, 12)
        }
    }
}

@MainActor
private struct LocationPreviewView: View {
    @Environment(\.dismiss) private var dismiss

    let location: ChatLocation
    @State private var cameraPosition: MapCameraPosition

    init(location: ChatLocation) {
        self.location = location
        _cameraPosition = State(
            initialValue: .region(
                MKCoordinateRegion(
                    center: CLLocationCoordinate2D(latitude: location.latitude, longitude: location.longitude),
                    span: MKCoordinateSpan(latitudeDelta: 0.012, longitudeDelta: 0.012)
                )
            )
        )
    }

    var body: some View {
        NavigationStack {
            Map(position: $cameraPosition) {
                Marker(
                    location.name,
                    coordinate: CLLocationCoordinate2D(latitude: location.latitude, longitude: location.longitude)
                )
            }
            .ignoresSafeArea(edges: .bottom)
            .navigationTitle(location.name)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button("关闭") { dismiss() }
                        .foregroundStyle(Color.nkPrimaryText)
                }

                ToolbarItem(placement: .topBarTrailing) {
                    Button("地图中打开") {
                        openInMaps()
                    }
                    .foregroundStyle(Color.nkPrimaryText)
                }
            }
        }
    }

    private func openInMaps() {
        let coordinate = CLLocationCoordinate2D(latitude: location.latitude, longitude: location.longitude)
        let placemark = MKPlacemark(coordinate: coordinate)
        let item = MKMapItem(placemark: placemark)
        item.name = location.name
        item.openInMaps()
    }
}
