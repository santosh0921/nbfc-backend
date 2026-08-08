import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:printing/printing.dart';
import 'package:provider/provider.dart';
import 'package:share_plus/share_plus.dart';
import 'dart:io';
import '../../../../core/components/app_card.dart';
import '../../../../core/components/app_scaffold.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/widgets/app_badge.dart';
import '../../domain/entities/visit_record.dart';
import '../providers/visit_history_provider.dart';

class VisitHistoryPage extends StatefulWidget {
  const VisitHistoryPage({super.key});

  @override
  State<VisitHistoryPage> createState() => _VisitHistoryPageState();
}

class _VisitHistoryPageState extends State<VisitHistoryPage> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => context.read<VisitHistoryProvider>().load());
  }

  @override
  Widget build(BuildContext context) {
    final records = context.watch<VisitHistoryProvider>().records;

    return AppScaffold(
      title: 'Visit History',
      subtitle: '${records.length} verification reports',
      padding: const EdgeInsets.all(AppSpacing.md),
      body: records.isEmpty
          ? Center(
              child: Text('No verification reports submitted yet', style: Theme.of(context).textTheme.bodyMedium),
            )
          : ListView.separated(
              itemCount: records.length,
              separatorBuilder: (_, __) => const SizedBox(height: AppSpacing.sm),
              itemBuilder: (context, index) => _HistoryCard(record: records[index]),
            ),
    );
  }
}

class _HistoryCard extends StatelessWidget {
  const _HistoryCard({required this.record});

  final VisitRecord record;

  AppBadgeVariant get _variant => switch (record.recommendation) {
        'Reject' => AppBadgeVariant.danger,
        'Request Additional Documents' => AppBadgeVariant.warning,
        _ => AppBadgeVariant.success,
      };

  @override
  Widget build(BuildContext context) {
    return AppCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(child: Text(record.customerName, style: Theme.of(context).textTheme.titleSmall)),
              AppBadge(label: record.recommendation, variant: _variant),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            '${record.loanType} · ${DateFormat('EEE, d MMM yyyy · h:mm a').format(record.submittedAt)}',
            style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary),
          ),
          if (record.pdfPath != null) ...[
            const SizedBox(height: AppSpacing.sm),
            Row(
              children: [
                TextButton.icon(
                  onPressed: () => Printing.layoutPdf(onLayout: (_) async => File(record.pdfPath!).readAsBytesSync()),
                  icon: const Icon(Icons.picture_as_pdf_outlined, size: 18),
                  label: const Text('View PDF'),
                ),
                const SizedBox(width: AppSpacing.sm),
                TextButton.icon(
                  onPressed: () => Share.shareXFiles([XFile(record.pdfPath!)], text: 'Verification report for ${record.customerName}'),
                  icon: const Icon(Icons.share_outlined, size: 18),
                  label: const Text('Share'),
                ),
              ],
            ),
          ],
        ],
      ),
    );
  }
}
