import 'dart:io';

import 'package:flutter/material.dart';
import 'package:printing/printing.dart';
import 'package:share_plus/share_plus.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../../core/widgets/success_celebration.dart';

/// Generic success screen shown after a recovery visit or recovery report is
/// submitted — offers a real "View PDF" / "Share PDF" pair when a PDF was
/// generated, mirroring VerificationSuccessPage's pattern.
class RecoveryDocumentSuccessPage extends StatelessWidget {
  const RecoveryDocumentSuccessPage({
    super.key,
    required this.title,
    required this.message,
    required this.pdfPath,
    required this.shareText,
  });

  final String title;
  final String message;
  final String? pdfPath;
  final String shareText;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.white,
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(AppSpacing.lg),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const SuccessCelebration(size: 88),
              const SizedBox(height: AppSpacing.lg),
              Text(title, style: Theme.of(context).textTheme.headlineSmall, textAlign: TextAlign.center),
              const SizedBox(height: AppSpacing.sm),
              Text(
                message,
                textAlign: TextAlign.center,
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: AppColors.textSecondary),
              ),
              const SizedBox(height: AppSpacing.xl),
              if (pdfPath != null) ...[
                AppButton(
                  label: 'View PDF Report',
                  icon: Icons.picture_as_pdf_outlined,
                  onPressed: () => Printing.layoutPdf(onLayout: (_) async => File(pdfPath!).readAsBytesSync()),
                ),
                const SizedBox(height: AppSpacing.sm),
                AppButton(
                  label: 'Share PDF Report',
                  icon: Icons.share_outlined,
                  variant: AppButtonVariant.outline,
                  onPressed: () => Share.shareXFiles([XFile(pdfPath!)], text: shareText),
                ),
                const SizedBox(height: AppSpacing.sm),
              ],
              AppButton(
                label: 'Done',
                variant: AppButtonVariant.text,
                onPressed: () => Navigator.of(context).popUntil((route) => route.isFirst),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
