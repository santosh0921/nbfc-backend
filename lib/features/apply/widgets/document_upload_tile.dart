import 'dart:io';
import 'package:flutter/material.dart';
import 'package:image_cropper/image_cropper.dart';
import 'package:image_picker/image_picker.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';
import '../../../core/widgets/premium_card.dart';

enum DocStatus { notUploaded, verifying, verified }

/// A single KYC document row: tap to pick a photo (camera or gallery),
/// crop it, then simulates a brief verification pass. Reports its final
/// verified state up via [onStatusChanged] so the parent step can gate
/// "Continue" on every required document being verified.
class DocumentUploadTile extends StatefulWidget {
  const DocumentUploadTile({
    super.key,
    required this.label,
    required this.icon,
    required this.onStatusChanged,
    this.onFileCaptured,
    this.color = AppColors.primary,
  });

  final String label;
  final IconData icon;
  final ValueChanged<bool> onStatusChanged;

  /// Each document type gets its own recognizable color (Aadhaar orange,
  /// PAN blue, electricity yellow, ...) so the icon+color combination is
  /// distinguishable at a glance without having to read the label —
  /// deliberately for customers who may not read comfortably.
  final Color color;

  /// Reports the cropped image file once captured — used by the KYC step
  /// to submit the selfie/photograph to the backend as base64.
  final ValueChanged<File>? onFileCaptured;

  @override
  State<DocumentUploadTile> createState() => _DocumentUploadTileState();
}

class _DocumentUploadTileState extends State<DocumentUploadTile> {
  File? _image;
  DocStatus _status = DocStatus.notUploaded;

  Future<void> _pick(ImageSource source) async {
    Navigator.of(context).pop(); // close the bottom sheet
    try {
      final picked = await ImagePicker().pickImage(source: source, imageQuality: 90);
      if (picked == null) return;

      final cropped = await ImageCropper().cropImage(
        sourcePath: picked.path,
        uiSettings: [
          AndroidUiSettings(
            toolbarTitle: 'Crop ${widget.label}',
            toolbarColor: AppColors.primary,
            toolbarWidgetColor: Colors.white,
            statusBarColor: AppColors.primaryDark,
            initAspectRatio: CropAspectRatioPreset.original,
            lockAspectRatio: false,
          ),
          IOSUiSettings(title: 'Crop ${widget.label}'),
        ],
      );
      if (cropped == null) return;

      setState(() {
        _image = File(cropped.path);
        _status = DocStatus.verifying;
      });
      widget.onFileCaptured?.call(_image!);

      await Future<void>.delayed(const Duration(milliseconds: 1100));
      if (!mounted) return;
      setState(() => _status = DocStatus.verified);
      widget.onStatusChanged(true);
    } catch (_) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Could not access camera/gallery for ${widget.label}.')),
      );
    }
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
              onTap: () => _pick(ImageSource.camera),
            ),
            ListTile(
              leading: const Icon(Icons.photo_library_rounded, color: AppColors.primary),
              title: const Text('Choose from Gallery'),
              onTap: () => _pick(ImageSource.gallery),
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
      onTap: _showSourceSheet,
      child: Row(
        children: [
          if (_image != null)
            ClipRRect(
              borderRadius: BorderRadius.circular(AppRadius.sm),
              child: Image.file(_image!, width: 44, height: 44, fit: BoxFit.cover),
            )
          else
            Container(
              width: 48,
              height: 48,
              decoration: BoxDecoration(
                color: widget.color.withValues(alpha: 0.12),
                borderRadius: BorderRadius.circular(AppRadius.sm),
                border: Border.all(color: widget.color.withValues(alpha: 0.3)),
              ),
              child: Icon(widget.icon, color: widget.color, size: 26),
            ),
          const SizedBox(width: 14),
          Expanded(child: Text(widget.label, style: theme.textTheme.titleSmall)),
          _StatusChip(status: _status),
        ],
      ),
    );
  }
}

class _StatusChip extends StatelessWidget {
  const _StatusChip({required this.status});

  final DocStatus status;

  @override
  Widget build(BuildContext context) {
    switch (status) {
      case DocStatus.notUploaded:
        return Text('Tap to upload', style: Theme.of(context).textTheme.labelSmall?.copyWith(color: AppColors.textSecondaryLight));
      case DocStatus.verifying:
        return Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            const SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2, color: AppColors.warning)),
            const SizedBox(width: 6),
            Text('Verifying…', style: Theme.of(context).textTheme.labelSmall?.copyWith(color: AppColors.warning)),
          ],
        );
      case DocStatus.verified:
        return Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.check_circle_rounded, size: 16, color: AppColors.success),
            const SizedBox(width: 4),
            Text('Verified', style: Theme.of(context).textTheme.labelSmall?.copyWith(color: AppColors.success, fontWeight: FontWeight.w700)),
          ],
        );
    }
  }
}
