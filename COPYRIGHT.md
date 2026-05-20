# Copyright

Copyright (c) 2026 ngosangns. All rights reserved.

## License Status

Repo này hiện chưa có `LICENSE` file và không cấp open-source license mặc định. Việc xem, clone hoặc chạy source theo các cơ chế công khai của nền tảng lưu trữ không đồng nghĩa với quyền sao chép, phân phối lại, sửa đổi hoặc tái cấp phép ngoài phạm vi được chủ sở hữu cho phép bằng văn bản.

Nếu sau này repo được phát hành theo một license cụ thể, file `LICENSE` ở root sẽ là nguồn tham chiếu chính và sẽ thay thế phần mô tả license status ở đây khi có mâu thuẫn.

## Third-Party Code And Packages

Các dependency và package bên thứ ba giữ nguyên license của tác giả hoặc nhà phát hành tương ứng. Tham chiếu kỹ thuật chính nằm trong:

- `go.mod` và `go.sum` cho Go modules.
- `package.json` và `package-lock.json` cho npm packages.
- CDN imports trong `internal/preview/preview_ui_src/index.html` và các file frontend liên quan.

Việc dùng dependency bên thứ ba trong repo này không chuyển quyền sở hữu hoặc license của các dependency đó sang chủ sở hữu repo.

## Presets And Generated Configuration

Nội dung trong `presets/` được embed và materialize sang các thư mục cấu hình user-level khi chạy CLI. Các file được tool tạo ra từ preset vẫn chịu cùng giới hạn bản quyền của repo này, trừ khi nội dung file đó ghi rõ license hoặc điều khoản khác.
