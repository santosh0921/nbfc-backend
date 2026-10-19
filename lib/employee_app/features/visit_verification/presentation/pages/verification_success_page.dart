import 'dart:io';

import 'package:flutter/material.dart';
import 'package:printing/printing.dart';
import 'package:share_plus/share_plus.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../../core/widgets/success_celebration.dart';
import '../../domain/entities/visit_record.dart';

class VerificationSuccessPage extends StatelessWidget {
  const VerificationSuccessPage({super.key, required this.record, this.reportUploadFailed = false});

  final VisitRecord record;

  /// True if the signed PDF couldn't be synced to the backend, so it
  /// won't show up in the admin panel yet — surfaced here rather than
  /// silently swallowed, since the whole point of generating this report
  /// is for admin to see it.
  final bool reportUploadFailed;

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
              Text('Verification Submitted', style: Theme.of(context).textTheme.headlineSmall),
              const SizedBox(height: AppSpacing.sm),
              Text(
                'Report for ${record.customerName} has been saved to your visit history.',
                textAlign: TextAlign.center,
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: AppColors.textSecondary),
              ),
              if (reportUploadFailed) ...[
                const SizedBox(height: AppSpacing.sm),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
                  decoration: BoxDecoration(
                    color: AppColors.warning.withValues(alpha: 0.1),
                    borderRadius: BorderRadius.circular(AppSpacing.radiusSm),
                    border: Border.all(color: AppColors.warning.withValues(alpha: 0.3)),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const Icon(Icons.cloud_off_outlined, size: 16, color: AppColors.warning),
                      const SizedBox(width: 6),
                      Flexible(
                        child: Text(
                          'Could not sync this report to admin yet — it\'s saved on this device, retry when back online.',
                          style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.warning),
                        ),
                      ),
                    ],
                  ),
                ),
              ],
              const SizedBox(height: AppSpacing.xl),
              if (record.pdfPath != null) ...[
                AppButton(
                  label: 'View PDF Report',
                  icon: Icons.picture_as_pdf_outlined,
                  onPressed: () => Printing.layoutPdf(
                    onLayout: (_) async => File(record.pdfPath!).readAsBytesSync(),
                  ),
                ),
                const SizedBox(height: AppSpacing.sm),
                AppButton(
                  label: 'Share PDF Report',
                  icon: Icons.share_outlined,
                  variant: AppButtonVariant.outline,
                  onPressed: () => SharePlus.instance.share(ShareParams(files: [XFile(record.pdfPath!)], text: 'Verification report for ${record.customerName}')),
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
