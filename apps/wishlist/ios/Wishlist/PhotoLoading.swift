import PhotosUI
import SwiftUI
import WishlistCore

/// 選んだ写真を、API が受け取る形式(JPEG)の `ImageUpload` にする。HEIC などもここで JPEG になる。
func loadImageUpload(from selection: PhotosPickerItem?) async -> ImageUpload? {
    guard let selection, let data = try? await selection.loadTransferable(type: Data.self),
        let image = UIImage(data: data), let jpeg = image.jpegData(compressionQuality: 0.9)
    else { return nil }
    return ImageUpload(data: jpeg, filename: "photo.jpg", contentType: "image/jpeg")
}
