import 'dart:io';

import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'package:provider/provider.dart';
import '../../../../core/components/app_card.dart';
import '../../../../core/components/app_scaffold.dart';
import '../../../../core/components/app_snackbar.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../core/widgets/app_text_field.dart';
import '../../domain/entities/recovery_case_entity.dart';
import '../providers/recovery_cases_provider.dart';
import '../providers/recovery_report_provider.dart';
import '../utils/recovery_pdf_generator.dart';
import 'recovery_document_success_page.dart';

class RecoveryReportPage extends StatefulWidget {
  const RecoveryReportPage({super.key, required this.caseId});

  final String caseId;

  @override
  State<RecoveryReportPage> createState() => _RecoveryReportPageState();
}

class _RecoveryReportPageState extends State<RecoveryReportPage> {
  final _remarksController = TextEditingController();
  final _recommendationController = TextEditingController();
  final _picker = ImagePicker();
  bool _capturing = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => context.read<RecoveryReportProvider>().reset());
  }

  @override
  void dispose() {
    _remarksController.dispose();
    _recommendationController.dispose();
    super.dispose();
  }

  Future<void> _capturePhoto() async {
    setState(() => _capturing = true);
    try {
      final photo = await _picker.pickImage(source: ImageSource.camera, imageQuality: 70);
      if (photo != null && mounted) context.read<RecoveryReportProvider>().addPhoto(photo.path);
    } catch (_) {
      if (mounted) AppSnackbar.danger(context, 'Could not capture photo');
    } finally {
      if (mounted) setState(() => _capturing = false);
    }
  }

  Future<void> _captureDocument() async {
    setState(() => _capturing = true);
    try {
      final photo = await _picker.pickImage(source: ImageSource.camera, imageQuality: 70);
      if (photo != null && mounted) context.read<RecoveryReportProvider>().addDocument(photo.path);
    } catch (_) {
      if (mounted) AppSnackbar.danger(context, 'Could not capture document');
    } finally {
      if (mounted) setState(() => _capturing = false);
    }
  }

  Future<void> _pickNextAction() async {
    final report = context.read<RecoveryReportProvider>();
    final date = await showDatePicker(
      context: context,
      initialDate: report.nextAction,
      firstDate: DateTime.now(),
      lastDate: DateTime.now().add(const Duration(days: 90)),
    );
    if (date != null) report.setNextAction(date);
  }

  Future<void> _submit() async {
    final report = context.read<RecoveryReportProvider>();
    final casesProvider = context.read<RecoveryCasesProvider>();
    report.setRemarks(_remarksController.text.trim());
    report.setRecommendation(_recommendationController.text.trim());
    var built = report.build();

    final caseItem = casesProvider.selectedCase;
    if (caseItem != null) {
      try {
        final pdfFile = await RecoveryPdfGenerator.generateReportPdf(caseItem, built);
        built = built.copyWith(pdfPath: pdfFile.path);
      } catch (_) {
        // PDF generation is best-effort; the report is still submitted without it.
      }
    }

    final note = built.remarks.isNotEmpty ? built.remarks : 'Recovery report submitted with recommendation: ${built.recommendation}.';
    await casesProvider.submitReport(
      widget.caseId,
      statusUpdate: built.status.label,
      remarks: note,
      riskAssessment: built.riskAssessment.label,
      recommendation: built.recommendation,
      nextAction: built.nextAction.toIso8601String(),
      newStatus: built.status,
      actionLabel: 'Report Submitted',
    );
    if (!mounted) return;
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(
        builder: (_) => RecoveryDocumentSuccessPage(
          title: 'Report Submitted',
          message: 'Recovery report has been saved${caseItem != null ? ' for ${caseItem.customerName}' : ''}.',
          pdfPath: built.pdfPath,
          shareText: 'Recovery report${caseItem != null ? ' for ${caseItem.customerName}' : ''}',
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final report = context.watch<RecoveryReportProvider>();

    return AppScaffold(
      title: 'Recovery Report',
      padding: const EdgeInsets.all(AppSpacing.md),
      body: SingleChildScrollView(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            AppCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('Recovery Status', style: Theme.of(context).textTheme.titleSmall),
                  const SizedBox(height: AppSpacing.sm),
                  DropdownButtonFormField<RecoveryStatus>(
                    value: report.status,
                    isExpanded: true,
                    decoration: const InputDecoration(labelText: 'Status'),
                    items: RecoveryStatus.values.map((s) => DropdownMenuItem(value: s, child: Text(s.label))).toList(),
                    onChanged: (value) => context.read<RecoveryReportProvider>().setStatus(value ?? report.status),
                  ),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.md),
            AppCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('Remarks', style: Theme.of(context).textTheme.titleSmall),
                  const SizedBox(height: AppSpacing.sm),
                  AppTextField(label: 'Remarks', controller: _remarksController, prefixIcon: Icons.notes_outlined),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.md),
            AppCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('Risk Assessment', style: Theme.of(context).textTheme.titleSmall),
                  const SizedBox(height: AppSpacing.sm),
                  Wrap(
                    spacing: AppSpacing.sm,
                    children: RiskLevel.values.map((r) {
                      return ChoiceChip(
                        label: Text(r.label),
                        selected: report.riskAssessment == r,
                        onSelected: (_) => context.read<RecoveryReportProvider>().setRiskAssessment(r),
                      );
                    }).toList(),
                  ),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.md),
            AppCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('Priority', style: Theme.of(context).textTheme.titleSmall),
                  const SizedBox(height: AppSpacing.sm),
                  Wrap(
                    spacing: AppSpacing.sm,
                    children: RecoveryPriority.values.map((p) {
                      return ChoiceChip(
                        label: Text(p.label),
                        selected: report.priority == p,
                        onSelected: (_) => context.read<RecoveryReportProvider>().setPriority(p),
                      );
                    }).toList(),
                  ),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.md),
            AppCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('Recommendation', style: Theme.of(context).textTheme.titleSmall),
                  const SizedBox(height: AppSpacing.sm),
                  AppTextField(label: 'Recommendation', controller: _recommendationController, prefixIcon: Icons.lightbulb_outline),
                  const SizedBox(height: AppSpacing.md),
                  ListTile(
                    contentPadding: EdgeInsets.zero,
                    title: const Text('Next Action Date'),
                    subtitle: Text('${report.nextAction.day}/${report.nextAction.month}/${report.nextAction.year}'),
                    trailing: const Icon(Icons.calendar_today_outlined),
                    onTap: _pickNextAction,
                  ),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.md),
            AppCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('Supporting Photos', style: Theme.of(context).textTheme.titleSmall),
                  const SizedBox(height: AppSpacing.sm),
                  if (report.photoPaths.isNotEmpty)
                    GridView.builder(
                      shrinkWrap: true,
                      physics: const NeverScrollableScrollPhysics(),
                      itemCount: report.photoPaths.length,
                      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                        crossAxisCount: 3,
                        mainAxisSpacing: AppSpacing.sm,
                        crossAxisSpacing: AppSpacing.sm,
                      ),
                      itemBuilder: (context, index) => ClipRRect(
                        borderRadius: BorderRadius.circular(AppSpacing.radiusSm),
                        child: Image.file(
                          File(report.photoPaths[index]),
                          fit: BoxFit.cover,
                          errorBuilder: (context, error, stackTrace) => Container(
                            color: AppColors.surfaceMuted,
                            child: const Icon(Icons.image_outlined, color: AppColors.textSecondary),
                          ),
                        ),
                      ),
                    )
                  else
                    Text('No photos added yet', style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary)),
                  const SizedBox(height: AppSpacing.sm),
                  AppButton(
                    label: 'Add Photo',
                    icon: Icons.add_a_photo_outlined,
                    variant: AppButtonVariant.outline,
                    isLoading: _capturing,
                    onPressed: _capturePhoto,
                  ),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.md),
            AppCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('Documents', style: Theme.of(context).textTheme.titleSmall),
                  const SizedBox(height: AppSpacing.sm),
                  if (report.documentPaths.isEmpty)
                    Text('No documents added yet', style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary))
                  else
                    ...report.documentPaths.map((p) => Padding(
                          padding: const EdgeInsets.symmetric(vertical: 4),
                          child: Row(
                            children: [
                              const Icon(Icons.description_outlined, size: 18, color: AppColors.success),
                              const SizedBox(width: AppSpacing.sm),
                              Expanded(child: Text(p.split('/').last, maxLines: 1, overflow: TextOverflow.ellipsis)),
                            ],
                          ),
                        )),
                  const SizedBox(height: AppSpacing.sm),
                  AppButton(
                    label: 'Add Document',
                    icon: Icons.camera_alt_outlined,
                    variant: AppButtonVariant.outline,
                    isLoading: _capturing,
                    onPressed: _captureDocument,
                  ),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.lg),
            AppButton(label: 'Submit Report', isLoading: report.isSubmitting, onPressed: _submit),
            const SizedBox(height: AppSpacing.xxl),
          ],
        ),
      ),
    );
  }
}
