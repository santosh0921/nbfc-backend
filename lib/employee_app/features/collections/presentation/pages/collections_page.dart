import 'dart:io';

import 'package:flutter/material.dart';
import 'package:printing/printing.dart';
import 'package:provider/provider.dart';
import 'package:share_plus/share_plus.dart';
import '../../../../core/components/app_card.dart';
import '../../../../core/components/app_scaffold.dart';
import '../../../../core/components/app_snackbar.dart';
import '../../../../core/components/gradient_header.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/utils/receipt_pdf_generator.dart';
import '../../../../core/widgets/app_badge.dart';
import '../../../../core/widgets/app_button.dart';
import '../../domain/entities/collection_entity.dart';
import '../providers/collections_provider.dart';

class CollectionsPage extends StatefulWidget {
  const CollectionsPage({super.key});

  @override
  State<CollectionsPage> createState() => _CollectionsPageState();
}

class _CollectionsPageState extends State<CollectionsPage> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => context.read<CollectionsProvider>().load());
  }

  Future<void> _confirmPayment(CollectionEntity item) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Confirm Payment'),
        content: Text('Mark ₹${item.emiAmount.toStringAsFixed(0)} from ${item.customerName} as collected?'),
        actions: [
          TextButton(onPressed: () => Navigator.of(context).pop(false), child: const Text('Cancel')),
          FilledButton(onPressed: () => Navigator.of(context).pop(true), child: const Text('Confirm')),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;

    await context.read<CollectionsProvider>().confirmPayment(item.id);
    if (!mounted) return;
    AppSnackbar.success(context, 'Payment recorded for ${item.customerName}');
    _showReceipt(item);
  }

  Future<void> _showReceipt(CollectionEntity item) async {
    File? receiptFile;
    try {
      receiptFile = await ReceiptPdfGenerator.generatePaymentReceipt(
        receiptNumber: 'RCPT${DateTime.now().millisecondsSinceEpoch}',
        customerName: item.customerName,
        loanAccountNumber: item.loanAccountNumber,
        amount: item.emiAmount,
        paymentMode: 'Cash',
        date: DateTime.now(),
        officerName: 'Verification Officer',
      );
    } catch (_) {
      // Receipt PDF generation is best-effort; the dialog still shows without it.
    }
    if (!mounted) return;
    showDialog(
      context: context,
      builder: (context) => Dialog(
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppSpacing.radiusLg)),
        child: Padding(
          padding: const EdgeInsets.all(AppSpacing.lg),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(Icons.check_circle, color: AppColors.success, size: 48),
              const SizedBox(height: AppSpacing.md),
              Text('Payment Receipt', style: Theme.of(context).textTheme.titleLarge),
              const SizedBox(height: AppSpacing.md),
              _ReceiptRow(label: 'Customer', value: item.customerName),
              _ReceiptRow(label: 'Loan A/C', value: item.loanAccountNumber),
              _ReceiptRow(label: 'Amount', value: '₹${item.emiAmount.toStringAsFixed(0)}'),
              _ReceiptRow(label: 'Date', value: '${DateTime.now().day}/${DateTime.now().month}/${DateTime.now().year}'),
              const SizedBox(height: AppSpacing.lg),
              if (receiptFile != null) ...[
                Row(
                  children: [
                    Expanded(
                      child: AppButton(
                        label: 'Download',
                        icon: Icons.download_outlined,
                        variant: AppButtonVariant.outline,
                        onPressed: () => Printing.layoutPdf(onLayout: (_) async => receiptFile!.readAsBytesSync()),
                      ),
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: AppButton(
                        label: 'Share',
                        icon: Icons.share_outlined,
                        variant: AppButtonVariant.outline,
                        onPressed: () => Share.shareXFiles(
                          [XFile(receiptFile!.path)],
                          text: 'Payment receipt for ${item.customerName}',
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: AppSpacing.sm),
              ],
              AppButton(label: 'Done', onPressed: () => Navigator.of(context).pop()),
            ],
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final provider = context.watch<CollectionsProvider>();

    return AppScaffold(
      showAppBar: false,
      body: provider.isLoading
          ? const Center(child: CircularProgressIndicator())
          : RefreshIndicator(
              onRefresh: provider.load,
              child: CustomScrollView(
                physics: const AlwaysScrollableScrollPhysics(),
                slivers: [
                  SliverToBoxAdapter(
                    child: GradientHeader(
                      title: 'Collections',
                      subtitle: "Today's EMI collection summary",
                      bottom: Row(
                        children: [
                          Expanded(
                            child: _SummaryTile(
                              label: 'Collected',
                              value: '₹${provider.todaysTotal.toStringAsFixed(0)}',
                              icon: Icons.check_circle_outline,
                            ),
                          ),
                          const SizedBox(width: AppSpacing.sm),
                          Expanded(
                            child: _SummaryTile(
                              label: 'Pending',
                              value: '${provider.pending.length}',
                              icon: Icons.hourglass_empty,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                  SliverPadding(
                    padding: const EdgeInsets.all(AppSpacing.md),
                    sliver: SliverList(
                      delegate: SliverChildListDelegate([
                        Text('Pending Collections', style: Theme.of(context).textTheme.titleMedium),
                        const SizedBox(height: AppSpacing.sm),
                        ...provider.items.map((item) => Padding(
                              padding: const EdgeInsets.only(bottom: AppSpacing.sm),
                              child: _CollectionCard(item: item, onCollect: () => _confirmPayment(item)),
                            )),
                      ]),
                    ),
                  ),
                ],
              ),
            ),
    );
  }
}

class _SummaryTile extends StatelessWidget {
  const _SummaryTile({required this.label, required this.value, required this.icon});

  final String label;
  final String value;
  final IconData icon;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: AppColors.secondary.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(AppSpacing.radiusMd),
        border: Border.all(color: AppColors.secondary.withValues(alpha: 0.14)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, color: AppColors.secondary, size: 20),
          const SizedBox(height: 6),
          Text(value, style: const TextStyle(color: AppColors.secondary, fontSize: 18, fontWeight: FontWeight.w800)),
          Text(label, style: TextStyle(color: AppColors.secondary.withValues(alpha: 0.7), fontSize: 12)),
        ],
      ),
    );
  }
}

class _ReceiptRow extends StatelessWidget {
  const _ReceiptRow({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: Theme.of(context).textTheme.bodySmall),
          Text(value, style: Theme.of(context).textTheme.bodyMedium?.copyWith(fontWeight: FontWeight.w600)),
        ],
      ),
    );
  }
}

class _CollectionCard extends StatelessWidget {
  const _CollectionCard({required this.item, required this.onCollect});

  final CollectionEntity item;
  final VoidCallback onCollect;

  @override
  Widget build(BuildContext context) {
    final isCollected = item.status == CollectionStatus.collected;
    final badgeVariant = switch (item.status) {
      CollectionStatus.collected => AppBadgeVariant.success,
      CollectionStatus.overdue => AppBadgeVariant.danger,
      CollectionStatus.pending => AppBadgeVariant.warning,
    };
    final badgeLabel = switch (item.status) {
      CollectionStatus.collected => 'Collected',
      CollectionStatus.overdue => 'Overdue',
      CollectionStatus.pending => 'Pending',
    };

    return AppCard(
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(item.customerName, style: Theme.of(context).textTheme.titleSmall),
                const SizedBox(height: 2),
                Text(item.loanAccountNumber, style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary)),
                const SizedBox(height: 6),
                Row(
                  children: [
                    AppBadge(label: badgeLabel, variant: badgeVariant),
                    const SizedBox(width: AppSpacing.sm),
                    Text('₹${item.emiAmount.toStringAsFixed(0)}', style: Theme.of(context).textTheme.titleSmall),
                  ],
                ),
              ],
            ),
          ),
          if (!isCollected)
            AppButton(label: 'Collect', expanded: false, size: AppButtonSize.small, onPressed: onCollect)
          else
            const Icon(Icons.receipt_long_outlined, color: AppColors.success),
        ],
      ),
    );
  }
}
