import 'dart:io';

import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:printing/printing.dart';
import 'package:provider/provider.dart';
import 'package:share_plus/share_plus.dart';
import 'package:url_launcher/url_launcher.dart';
import '../../../../core/components/app_bottom_sheet.dart';
import '../../../../core/components/app_card.dart';
import '../../../../core/components/app_snackbar.dart';
import '../../../../core/components/gradient_header.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/router/route_names.dart';
import '../../../../core/utils/demo_call.dart';
import '../../../../core/widgets/app_badge.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../core/widgets/app_text_field.dart';
import '../../data/repositories/collection_repository.dart';
import '../../data/repositories/payment_repository.dart';
import '../../domain/entities/recovery_case_entity.dart';
import '../providers/recovery_cases_provider.dart';
import '../utils/recovery_pdf_generator.dart';

class RecoveryCaseDetailPage extends StatefulWidget {
  const RecoveryCaseDetailPage({super.key, required this.caseId});

  final String caseId;

  @override
  State<RecoveryCaseDetailPage> createState() => _RecoveryCaseDetailPageState();
}

class _RecoveryCaseDetailPageState extends State<RecoveryCaseDetailPage> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => context.read<RecoveryCasesProvider>().loadCaseDetail(widget.caseId));
  }

  @override
  Widget build(BuildContext context) {
    final provider = context.watch<RecoveryCasesProvider>();
    final caseItem = provider.selectedCase;

    if (provider.isLoadingDetail || caseItem == null || caseItem.id != widget.caseId) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }

    return Scaffold(
      body: CustomScrollView(
        slivers: [
          SliverToBoxAdapter(
            child: GradientHeader(
              title: caseItem.customerName,
              subtitle: '${caseItem.loanId} · ${caseItem.loanType.label}',
              trailing: IconButton(
                onPressed: () => Navigator.of(context).pop(),
                icon: const Icon(Icons.arrow_back, color: AppColors.secondary),
              ),
            ),
          ),
          SliverPadding(
            padding: const EdgeInsets.all(AppSpacing.md),
            sliver: SliverList(
              delegate: SliverChildListDelegate([
                _CustomerInfoCard(caseItem: caseItem),
                const SizedBox(height: AppSpacing.md),
                _LoanSummaryCard(caseItem: caseItem),
                const SizedBox(height: AppSpacing.md),
                _EmiHistoryCard(entries: provider.emiHistory),
                const SizedBox(height: AppSpacing.md),
                _TimelineCard(events: provider.timeline),
                const SizedBox(height: AppSpacing.md),
                _RecoveryActionsCard(caseItem: caseItem),
                const SizedBox(height: AppSpacing.xxl),
              ]),
            ),
          ),
        ],
      ),
    );
  }
}

class _CustomerInfoCard extends StatelessWidget {
  const _CustomerInfoCard({required this.caseItem});

  final RecoveryCaseEntity caseItem;

  AppBadgeVariant get _statusVariant => switch (caseItem.status) {
        RecoveryStatus.fullPayment || RecoveryStatus.closed => AppBadgeVariant.success,
        RecoveryStatus.escalated || RecoveryStatus.legalRecommended || RecoveryStatus.fraudSuspicion => AppBadgeVariant.danger,
        RecoveryStatus.promiseToPay || RecoveryStatus.partialPayment || RecoveryStatus.settlement => AppBadgeVariant.gold,
        RecoveryStatus.visitScheduled || RecoveryStatus.visited => AppBadgeVariant.warning,
        RecoveryStatus.unableToLocate => AppBadgeVariant.neutral,
        RecoveryStatus.assigned || RecoveryStatus.contacted => AppBadgeVariant.info,
      };

  @override
  Widget build(BuildContext context) {
    return AppCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Flexible(
                child: Text(
                  'Customer Information',
                  style: Theme.of(context).textTheme.titleMedium,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              const Spacer(),
              AppBadge(label: caseItem.status.label, variant: _statusVariant),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          _DetailRow(icon: Icons.location_on_outlined, label: 'Address', value: caseItem.address),
          _DetailRow(icon: Icons.phone_outlined, label: 'Phone', value: caseItem.customerPhone),
          _DetailRow(icon: Icons.apartment_outlined, label: 'Branch', value: caseItem.branch),
          _DetailRow(
            icon: Icons.event_outlined,
            label: 'Assigned',
            value: '${caseItem.assignedDate.day}/${caseItem.assignedDate.month}/${caseItem.assignedDate.year}',
          ),
          const SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Expanded(
                child: AppButton(
                  label: 'Call',
                  icon: Icons.call_outlined,
                  onPressed: () => showCallOutcomeFlow(
                    context,
                    name: caseItem.customerName,
                    phone: caseItem.customerPhone,
                    onProceed: () {},
                  ),
                ),
              ),
              const SizedBox(width: AppSpacing.sm),
              Expanded(
                child: AppButton(
                  label: 'Navigate',
                  icon: Icons.navigation_outlined,
                  variant: AppButtonVariant.outline,
                  onPressed: () => launchUrl(
                    Uri.parse('https://www.google.com/maps/search/?api=1&query=${Uri.encodeComponent(caseItem.address)}'),
                    mode: LaunchMode.externalApplication,
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _LoanSummaryCard extends StatelessWidget {
  const _LoanSummaryCard({required this.caseItem});

  final RecoveryCaseEntity caseItem;

  @override
  Widget build(BuildContext context) {
    return AppCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Loan Summary', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: AppSpacing.md),
          _DetailRow(icon: Icons.currency_rupee, label: 'Outstanding', value: '₹${caseItem.outstandingAmount.toStringAsFixed(0)}'),
          _DetailRow(icon: Icons.warning_amber_outlined, label: 'Penalty', value: '₹${caseItem.penaltyAmount.toStringAsFixed(0)}'),
          _DetailRow(icon: Icons.percent_outlined, label: 'Interest', value: '₹${caseItem.interestAmount.toStringAsFixed(0)}'),
          _DetailRow(icon: Icons.receipt_long_outlined, label: 'Overdue EMIs', value: '${caseItem.overdueEmiCount}'),
          _DetailRow(icon: Icons.event_busy_outlined, label: 'DPD', value: '${caseItem.dpd} days'),
        ],
      ),
    );
  }
}

class _EmiHistoryCard extends StatelessWidget {
  const _EmiHistoryCard({required this.entries});

  final List<EmiHistoryEntry> entries;

  @override
  Widget build(BuildContext context) {
    return AppCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('EMI History', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: AppSpacing.sm),
          if (entries.isEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
              child: Text('No EMI history available', style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary)),
            )
          else
            ...entries.map((e) => Padding(
                  padding: const EdgeInsets.symmetric(vertical: 6),
                  child: Row(
                    children: [
                      Icon(
                        e.paid ? Icons.check_circle : Icons.error_outline,
                        size: 18,
                        color: e.paid ? AppColors.success : AppColors.danger,
                      ),
                      const SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: Text(e.month, style: Theme.of(context).textTheme.bodyMedium, maxLines: 1, overflow: TextOverflow.ellipsis),
                      ),
                      Flexible(
                        child: Text(
                          '₹${e.amount.toStringAsFixed(0)}',
                          style: Theme.of(context).textTheme.bodyMedium,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      const SizedBox(width: AppSpacing.sm),
                      AppBadge(label: e.paid ? 'Paid' : 'Overdue', variant: e.paid ? AppBadgeVariant.success : AppBadgeVariant.danger),
                    ],
                  ),
                )),
        ],
      ),
    );
  }
}

class _TimelineCard extends StatelessWidget {
  const _TimelineCard({required this.events});

  final List<RecoveryTimelineEvent> events;

  @override
  Widget build(BuildContext context) {
    return AppCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Recovery Timeline', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: AppSpacing.sm),
          if (events.isEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
              child: Text('No activity recorded yet', style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary)),
            )
          else
            for (int i = 0; i < events.length; i++)
              IntrinsicHeight(
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Column(
                      children: [
                        Container(
                          width: 10,
                          height: 10,
                          decoration: BoxDecoration(
                            color: i == events.length - 1 ? AppColors.gold : AppColors.primary,
                            shape: BoxShape.circle,
                          ),
                        ),
                        if (i != events.length - 1) Expanded(child: Container(width: 2, color: AppColors.divider)),
                      ],
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: Padding(
                        padding: const EdgeInsets.only(bottom: AppSpacing.md),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(events[i].title, style: Theme.of(context).textTheme.titleSmall),
                            const SizedBox(height: 2),
                            Text(events[i].description, style: Theme.of(context).textTheme.bodySmall),
                            const SizedBox(height: 2),
                            Text(
                              '${events[i].timestamp.day}/${events[i].timestamp.month}/${events[i].timestamp.year}',
                              style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ],
                ),
              ),
        ],
      ),
    );
  }
}

class _DetailRow extends StatelessWidget {
  const _DetailRow({required this.icon, required this.label, required this.value});

  final IconData icon;
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 18, color: AppColors.textSecondary),
          const SizedBox(width: AppSpacing.sm),
          SizedBox(width: 96, child: Text(label, style: Theme.of(context).textTheme.bodySmall)),
          Expanded(child: Text(value, style: Theme.of(context).textTheme.bodyMedium)),
        ],
      ),
    );
  }
}

class _RecoveryActionsCard extends StatelessWidget {
  const _RecoveryActionsCard({required this.caseItem});

  final RecoveryCaseEntity caseItem;

  Future<void> _confirmAction(
    BuildContext context, {
    required String title,
    required String message,
    required RecoveryStatus status,
    required String actionLabel,
  }) async {
    final remarksController = TextEditingController();
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(title),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(message),
            const SizedBox(height: AppSpacing.md),
            AppTextField(label: 'Remarks (optional)', controller: remarksController),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.of(context).pop(false), child: const Text('Cancel')),
          FilledButton(onPressed: () => Navigator.of(context).pop(true), child: const Text('Confirm')),
        ],
      ),
    );
    if (confirmed != true || !context.mounted) return;
    await context.read<RecoveryCasesProvider>().performAction(caseItem.id, status, actionLabel, note: remarksController.text.trim());
    if (context.mounted) AppSnackbar.success(context, '$actionLabel logged');
  }

  Future<void> _scheduleVisit(BuildContext context) async {
    final date = await showDatePicker(
      context: context,
      initialDate: DateTime.now().add(const Duration(days: 1)),
      firstDate: DateTime.now(),
      lastDate: DateTime.now().add(const Duration(days: 60)),
    );
    if (date == null || !context.mounted) return;
    await context.read<RecoveryCasesProvider>().performAction(
          caseItem.id,
          RecoveryStatus.visitScheduled,
          'Visit Scheduled',
          note: 'Visit scheduled for ${date.day}/${date.month}/${date.year}.',
        );
    if (!context.mounted) return;
    AppSnackbar.success(context, 'Visit scheduled for ${date.day}/${date.month}/${date.year}');
  }

  Future<void> _promiseToPay(BuildContext context) async {
    final amountController = TextEditingController();
    final remarksController = TextEditingController();
    DateTime promisedDate = DateTime.now().add(const Duration(days: 3));
    DateTime reminderDate = DateTime.now().add(const Duration(days: 2));

    await AppBottomSheet.show(
      context,
      title: 'Promise To Pay',
      child: StatefulBuilder(
        builder: (context, setSheetState) {
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              AppTextField(label: 'Promised Amount (₹)', controller: amountController, keyboardType: TextInputType.number),
              const SizedBox(height: AppSpacing.md),
              ListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('Promised Date'),
                subtitle: Text('${promisedDate.day}/${promisedDate.month}/${promisedDate.year}'),
                trailing: const Icon(Icons.calendar_today_outlined),
                onTap: () async {
                  final picked = await showDatePicker(
                    context: context,
                    initialDate: promisedDate,
                    firstDate: DateTime.now(),
                    lastDate: DateTime.now().add(const Duration(days: 90)),
                  );
                  if (picked != null) setSheetState(() => promisedDate = picked);
                },
              ),
              ListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('Reminder Date'),
                subtitle: Text('${reminderDate.day}/${reminderDate.month}/${reminderDate.year}'),
                trailing: const Icon(Icons.notifications_active_outlined),
                onTap: () async {
                  final picked = await showDatePicker(
                    context: context,
                    initialDate: reminderDate,
                    firstDate: DateTime.now(),
                    lastDate: DateTime.now().add(const Duration(days: 90)),
                  );
                  if (picked != null) setSheetState(() => reminderDate = picked);
                },
              ),
              const SizedBox(height: AppSpacing.sm),
              AppTextField(label: 'Remarks', controller: remarksController),
              const SizedBox(height: AppSpacing.lg),
              AppButton(
                label: 'Save Promise',
                onPressed: () async {
                  final amount = double.tryParse(amountController.text.trim()) ?? 0;
                  if (amount <= 0) {
                    AppSnackbar.danger(context, 'Enter a valid amount');
                    return;
                  }
                  await context.read<CollectionRepositoryMock>().recordPromiseToPay(
                        caseId: caseItem.id,
                        promisedAmount: amount,
                        promisedDate: promisedDate,
                        remarks: remarksController.text.trim(),
                        reminderDate: reminderDate,
                      );
                  if (!context.mounted) return;
                  await context.read<RecoveryCasesProvider>().performAction(
                        caseItem.id,
                        RecoveryStatus.promiseToPay,
                        'Promise To Pay',
                        note: 'Customer promised ₹${amount.toStringAsFixed(0)} by ${promisedDate.day}/${promisedDate.month}/${promisedDate.year}.',
                      );
                  if (context.mounted) Navigator.of(context).pop();
                  if (context.mounted) AppSnackbar.success(context, 'Promise to pay recorded');
                },
              ),
            ],
          );
        },
      ),
    );
  }

  Future<void> _recordPayment(BuildContext context, {required bool full}) async {
    final amountController = TextEditingController();
    final remarksController = TextEditingController();
    PaymentMode mode = PaymentMode.cash;

    await AppBottomSheet.show(
      context,
      title: full ? 'Full Payment' : 'Partial Payment',
      child: StatefulBuilder(
        builder: (context, setSheetState) {
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              AppTextField(label: 'Amount (₹)', controller: amountController, keyboardType: TextInputType.number),
              const SizedBox(height: AppSpacing.md),
              DropdownButtonFormField<PaymentMode>(
                value: mode,
                decoration: const InputDecoration(labelText: 'Payment Mode'),
                items: PaymentMode.values.map((m) => DropdownMenuItem(value: m, child: Text(m.label))).toList(),
                onChanged: (value) => setSheetState(() => mode = value ?? mode),
              ),
              const SizedBox(height: AppSpacing.md),
              AppTextField(label: 'Remarks', controller: remarksController),
              const SizedBox(height: AppSpacing.lg),
              AppButton(
                label: 'Record Payment',
                onPressed: () async {
                  final amount = double.tryParse(amountController.text.trim()) ?? 0;
                  if (amount <= 0) {
                    AppSnackbar.danger(context, 'Enter a valid amount');
                    return;
                  }
                  final record = await context.read<PaymentRepositoryMock>().recordPayment(
                        caseId: caseItem.id,
                        amount: amount,
                        mode: mode,
                        remarks: remarksController.text.trim(),
                      );
                  if (!context.mounted) return;
                  await context.read<RecoveryCasesProvider>().performAction(
                        caseItem.id,
                        full ? RecoveryStatus.fullPayment : RecoveryStatus.partialPayment,
                        full ? 'Full Payment' : 'Partial Payment',
                        note: '₹${amount.toStringAsFixed(0)} received via ${mode.label}. Receipt ${record.receiptNumber}.',
                      );
                  if (context.mounted) Navigator.of(context).pop();
                  File? receiptFile;
                  try {
                    receiptFile = await RecoveryPdfGenerator.generatePaymentReceiptPdf(
                      caseItem: caseItem,
                      receiptNumber: record.receiptNumber,
                      amount: record.amount,
                      paymentMode: record.mode.label,
                      date: record.recordedAt,
                    );
                  } catch (_) {
                    // Receipt PDF generation is best-effort.
                  }
                  if (context.mounted) {
                    await showDialog<void>(
                      context: context,
                      builder: (context) => AlertDialog(
                        title: const Text('Payment Recorded'),
                        content: Column(
                          mainAxisSize: MainAxisSize.min,
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text('Receipt Number: ${record.receiptNumber}'),
                            const SizedBox(height: 8),
                            Text('Amount: ₹${record.amount.toStringAsFixed(0)}'),
                            Text('Mode: ${record.mode.label}'),
                          ],
                        ),
                        actions: [
                          if (receiptFile != null) ...[
                            TextButton.icon(
                              onPressed: () => Printing.layoutPdf(onLayout: (_) async => receiptFile!.readAsBytesSync()),
                              icon: const Icon(Icons.download_outlined, size: 18),
                              label: const Text('Download'),
                            ),
                            TextButton.icon(
                              onPressed: () => Share.shareXFiles(
                                [XFile(receiptFile!.path)],
                                text: 'Payment receipt for ${caseItem.customerName}',
                              ),
                              icon: const Icon(Icons.share_outlined, size: 18),
                              label: const Text('Share'),
                            ),
                          ],
                          FilledButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Done')),
                        ],
                      ),
                    );
                  }
                },
              ),
            ],
          );
        },
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AppCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Recovery Actions', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: AppSpacing.md),
          Wrap(
            spacing: AppSpacing.sm,
            runSpacing: AppSpacing.sm,
            children: [
              AppButton(
                label: 'Contacted',
                expanded: false,
                size: AppButtonSize.small,
                icon: Icons.chat_outlined,
                onPressed: () => _confirmAction(
                  context,
                  title: 'Mark Customer Contacted',
                  message: 'Confirm the customer was contacted for this case.',
                  status: RecoveryStatus.contacted,
                  actionLabel: 'Customer Contacted',
                ),
              ),
              AppButton(
                label: 'Schedule Visit',
                expanded: false,
                size: AppButtonSize.small,
                icon: Icons.event_outlined,
                variant: AppButtonVariant.outline,
                onPressed: () => _scheduleVisit(context),
              ),
              AppButton(
                label: 'Visited Customer',
                expanded: false,
                size: AppButtonSize.small,
                icon: Icons.directions_walk_outlined,
                onPressed: () => context.push(RouteNames.recoveryVisitPath.replaceFirst(':id', caseItem.id)),
              ),
              AppButton(
                label: 'Not Available',
                expanded: false,
                size: AppButtonSize.small,
                icon: Icons.person_off_outlined,
                variant: AppButtonVariant.outline,
                onPressed: () => _confirmAction(
                  context,
                  title: 'Customer Not Available',
                  message: 'Log that the customer was not available at the visit.',
                  status: RecoveryStatus.contacted,
                  actionLabel: 'Customer Not Available',
                ),
              ),
              AppButton(
                label: 'Promise To Pay',
                expanded: false,
                size: AppButtonSize.small,
                icon: Icons.handshake_outlined,
                onPressed: () => _promiseToPay(context),
              ),
              AppButton(
                label: 'Partial Payment',
                expanded: false,
                size: AppButtonSize.small,
                icon: Icons.payments_outlined,
                onPressed: () => _recordPayment(context, full: false),
              ),
              AppButton(
                label: 'Full Payment',
                expanded: false,
                size: AppButtonSize.small,
                icon: Icons.paid_outlined,
                onPressed: () => _recordPayment(context, full: true),
              ),
              AppButton(
                label: 'Settlement',
                expanded: false,
                size: AppButtonSize.small,
                icon: Icons.balance_outlined,
                variant: AppButtonVariant.outline,
                onPressed: () => _confirmAction(
                  context,
                  title: 'Settlement Discussion',
                  message: 'Log a settlement discussion for this case.',
                  status: RecoveryStatus.settlement,
                  actionLabel: 'Settlement Discussion',
                ),
              ),
              AppButton(
                label: 'Escalate',
                expanded: false,
                size: AppButtonSize.small,
                icon: Icons.trending_up,
                variant: AppButtonVariant.outline,
                onPressed: () => _confirmAction(
                  context,
                  title: 'Escalate Case',
                  message: 'Escalate this case for higher-level review.',
                  status: RecoveryStatus.escalated,
                  actionLabel: 'Case Escalated',
                ),
              ),
              AppButton(
                label: 'Legal Recommendation',
                expanded: false,
                size: AppButtonSize.small,
                icon: Icons.gavel_outlined,
                variant: AppButtonVariant.danger,
                onPressed: () => _confirmAction(
                  context,
                  title: 'Legal Recommendation',
                  message: 'Recommend this case for legal action.',
                  status: RecoveryStatus.legalRecommended,
                  actionLabel: 'Legal Recommendation',
                ),
              ),
              AppButton(
                label: 'Unable To Locate',
                expanded: false,
                size: AppButtonSize.small,
                icon: Icons.person_search_outlined,
                variant: AppButtonVariant.outline,
                onPressed: () => _confirmAction(
                  context,
                  title: 'Unable To Locate',
                  message: 'Mark the customer as unable to be located.',
                  status: RecoveryStatus.unableToLocate,
                  actionLabel: 'Unable To Locate',
                ),
              ),
              AppButton(
                label: 'Fraud Suspicion',
                expanded: false,
                size: AppButtonSize.small,
                icon: Icons.report_problem_outlined,
                variant: AppButtonVariant.danger,
                onPressed: () => _confirmAction(
                  context,
                  title: 'Fraud Suspicion',
                  message: 'Flag this case for suspected fraud.',
                  status: RecoveryStatus.fraudSuspicion,
                  actionLabel: 'Fraud Suspicion Flagged',
                ),
              ),
              AppButton(
                label: 'Close Case',
                expanded: false,
                size: AppButtonSize.small,
                icon: Icons.task_alt_outlined,
                onPressed: () => _confirmAction(
                  context,
                  title: 'Case Closed',
                  message: 'Confirm closing this recovery case.',
                  status: RecoveryStatus.closed,
                  actionLabel: 'Case Closed',
                ),
              ),
              AppButton(
                label: 'Submit Report',
                expanded: false,
                size: AppButtonSize.small,
                icon: Icons.description_outlined,
                variant: AppButtonVariant.outline,
                onPressed: () => context.push(RouteNames.recoveryReportPath.replaceFirst(':id', caseItem.id)),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
