import 'dart:convert';
import 'dart:io';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import '../../../core/network/api_exception.dart';
import '../../../core/network/document_api_service.dart';
import '../../../core/providers/auth_provider.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';
import '../../../core/widgets/premium_card.dart';

enum _UploadState { idle, uploading, uploaded, failed }

/// A document upload row that — unlike [DocumentUploadTile], which is
/// image-only (camera/gallery, local crop+mock-verify, never leaves the
/// device until the whole application submits) — actually uploads the
/// file to the backend immediately via `POST /auth/documents`
/// (internal/documents/handler.go), and offers a third source: browsing
/// files, so a chequebook/passbook scan already saved as a PDF or photo
/// doesn't require re-taking it with the camera.
class BankDocumentUploadTile extends StatefulWidget {
  const BankDocumentUploadTile({super.key, required this.label, required this.icon, required this.docType});

  final String label;
  final IconData icon;

  /// Sent as the backend's `docType` field (e.g. "chequebook", "passbook").
  final String docType;

  @override
  State<BankDocumentUploadTile> createState() => _BankDocumentUploadTileState();
}

class _BankDocumentUploadTileState extends State<BankDocumentUploadTile> {
  _UploadState _state = _UploadState.idle;
  String? _fileName;
  String? _error;

  static const _allowedExtensions = ['jpg', 'jpeg', 'png', 'pdf'];

  Future<void> _uploadBytes(List<int> bytes, String fileName, String mimeType) async {
    final token = AuthProvider.instance.token;
    if (token == null) return;
    setState(() {
      _state = _UploadState.uploading;
      _fileName = fileName;
      _error = null;
    });
    try {
      await DocumentApiService.upload(
        token,
        docType: widget.docType,
        fileName: fileName,
        mimeType: mimeType,
        dataBase64: base64Encode(bytes),
      );
      if (!mounted) return;
      setState(() => _state = _UploadState.uploaded);
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _state = _UploadState.failed;
        _error = e.message;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _state = _UploadState.failed;
        _error = 'Upload failed. Please try again.';
      });
    }
  }

  Future<void> _pickFromCamera() async {
    Navigator.of(context).pop();
    final picked = await ImagePicker().pickImage(source: ImageSource.camera, imageQuality: 90);
    if (picked == null) return;
    await _uploadBytes(await File(picked.path).readAsBytes(), picked.name, 'image/jpeg');
  }

  Future<void> _pickFromGallery() async {
    Navigator.of(context).pop();
    final picked = await ImagePicker().pickImage(source: ImageSource.gallery, imageQuality: 90);
    if (picked == null) return;
    await _uploadBytes(await File(picked.path).readAsBytes(), picked.name, 'image/jpeg');
  }

  Future<void> _pickFromFiles() async {
    Navigator.of(context).pop();
    final result = await FilePicker.pickFiles(
      type: FileType.custom,
      allowedExtensions: _allowedExtensions,
      withData: true,
    );
    final picked = result?.files.single;
    if (picked == null) return;
    final bytes = picked.bytes ?? (picked.path != null ? await File(picked.path!).readAsBytes() : null);
    if (bytes == null) return;
    final ext = (picked.extension ?? 'pdf').toLowerCase();
    final mimeType = ext == 'pdf' ? 'application/pdf' : (ext == 'png' ? 'image/png' : 'image/jpeg');
    await _uploadBytes(bytes, picked.name, mimeType);
  }

  void _showSourceSheet() {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: Colors.transparent,
      builder: (context) => Container(
        padding: const EdgeInsets.fromLTRB(20, 12, 20, 28),
        decoration: const BoxDecoration(
          color: AppColors.surfaceLight,
          borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.xl)),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(width: 40, height: 4, decoration: BoxDecoration(color: AppColors.borderLight, borderRadius: BorderRadius.circular(4))),
            const SizedBox(height: 16),
            Text('Upload ${widget.label}', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 16),
            ListTile(
              leading: const Icon(Icons.photo_camera_rounded, color: AppColors.primary),
              title: const Text('Take Photo'),
              onTap: _pickFromCamera,
            ),
            ListTile(
              leading: const Icon(Icons.photo_library_rounded, color: AppColors.primary),
              title: const Text('Choose from Gallery'),
              onTap: _pickFromGallery,
            ),
            ListTile(
              leading: const Icon(Icons.folder_open_rounded, color: AppColors.primary),
              title: const Text('Browse Files'),
              subtitle: const Text('Image or PDF'),
              onTap: _pickFromFiles,
            ),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return PremiumCard(
      onTap: _state == _UploadState.uploading ? null : _showSourceSheet,
      child: Row(
        children: [
          Container(
            width: 44,
            height: 44,
            decoration: BoxDecoration(
              color: AppColors.primary.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(AppRadius.sm),
            ),
            child: Icon(widget.icon, color: AppColors.primary),
          ),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(widget.label, style: theme.textTheme.titleSmall, maxLines: 1, overflow: TextOverflow.ellipsis),
                if (_fileName != null)
                  Text(_fileName!, style: theme.textTheme.bodySmall, maxLines: 1, overflow: TextOverflow.ellipsis),
                if (_state == _UploadState.failed && _error != null)
                  Text(_error!, style: theme.textTheme.bodySmall?.copyWith(color: AppColors.error), maxLines: 2, overflow: TextOverflow.ellipsis),
              ],
            ),
          ),
          const SizedBox(width: 8),
          _StatusIndicator(state: _state),
        ],
      ),
    );
  }
}

class _StatusIndicator extends StatelessWidget {
  const _StatusIndicator({required this.state});

  final _UploadState state;

  @override
  Widget build(BuildContext context) {
    switch (state) {
      case _UploadState.idle:
        return Text('Optional', style: Theme.of(context).textTheme.labelSmall?.copyWith(color: AppColors.textSecondaryLight));
      case _UploadState.uploading:
        return const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2, color: AppColors.warning));
      case _UploadState.uploaded:
        return const Icon(Icons.check_circle_rounded, size: 20, color: AppColors.success);
      case _UploadState.failed:
        return const Icon(Icons.error_rounded, size: 20, color: AppColors.error);
    }
  }
}
