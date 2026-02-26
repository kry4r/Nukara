import SwiftUI

struct AvatarView: View {
    let name: String
    let imageData: Data?
    let size: CGFloat

    var body: some View {
        Group {
            if let imageData,
               let uiImage = UIImage(data: imageData) {
                Image(uiImage: uiImage)
                    .resizable()
                    .scaledToFill()
            } else {
                Circle()
                    .fill(Color.nkAccent.opacity(0.25))
                    .overlay(
                        Text(String(name.prefix(1)))
                            .font(.system(size: size * 0.45, weight: .semibold))
                            .foregroundStyle(Color.nkPrimaryText)
                    )
            }
        }
        .frame(width: size, height: size)
        .clipShape(Circle())
        .overlay(
            Circle().stroke(Color.white.opacity(0.65), lineWidth: 1)
        )
    }
}
