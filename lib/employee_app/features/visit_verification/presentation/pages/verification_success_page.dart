import 'dart:io';

import 'package:flutter/material.dart';
import 'package:printing/printing.dart';
import 'package:share_plus/share_plus.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/widgets/app_button.dart';
import '../../domain/entities/visit_record.dart';

class VerificationSuccessPage extends StatelessWidget {
  const VerificationSuccessPage({super.key, required this.record});

  final VisitRecord record;

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
              Container(
                width: 88,
                height: 88,
                alignment: Alignment.center,
                decoration: BoxDecoration(color: AppColors.success.withValues(alpha: 0.12), shape: BoxShape.circle),
                child: const Icon(Icons.check_circle, color: AppColors.success, size: 56),
              ),
              const SizedBox(height: AppSpacing.lg),
              Text('Verification Submitted', style: Theme.of(context).textTheme.headlineSmall),
              const SizedBox(height: AppSpacing.sm),
              Text(
                'Report for ${record.customerName} has been saved to your visit history.',
                textAlign: TextAlign.center,
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: AppColors.textSecondary),
              ),
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
                  onPressed: () => Share.shareXFiles([XFile(record.pdfPath!)], text: 'Verification report for ${record.customerName}'),
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
