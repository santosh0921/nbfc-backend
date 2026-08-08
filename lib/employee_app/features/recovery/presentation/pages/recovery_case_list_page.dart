import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';
import '../../../../core/components/app_bottom_sheet.dart';
import '../../../../core/components/app_card.dart';
import '../../../../core/components/app_filter_chips.dart';
import '../../../../core/components/app_scaffold.dart';
import '../../../../core/components/app_search_bar.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/router/route_names.dart';
import '../../../../core/utils/demo_call.dart';
import '../../../../core/widgets/app_badge.dart';
import '../../../../core/widgets/app_button.dart';
import '../../domain/entities/recovery_case_entity.dart';
import '../providers/recovery_cases_provider.dart';

class RecoveryCaseListPage extends StatefulWidget {
  const RecoveryCaseListPage({super.key});

  @override
  State<RecoveryCaseListPage> createState() => _RecoveryCaseListPageState();
}

class _RecoveryCaseListPageState extends State<RecoveryCaseListPage> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => context.read<RecoveryCasesProvider>().load());
  }

  bool get _hasActiveFilters {
    final p = context.read<RecoveryCasesProvider>();
    return p.priorityFilter != null || p.riskFilter != null || p.statusFilter != null;
  }

  void _openFilters() {
    final provider = context.read<RecoveryCasesProvider>();
    AppBottomSheet.show(
      context,
      title: 'Filter Cases',
      child: StatefulBuilder(
        builder: (context, setSheetState) {
          return SingleChildScrollView(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('Priority', style: Theme.of(context).textTheme.titleSmall),
                const SizedBox(height: AppSpacing.sm),
                Wrap(
                  spacing: AppSpacing.sm,
                  children: RecoveryPriority.values.map((p) {
                    return AppFilterChip(
                      label: p.label,
                      selected: provider.priorityFilter == p,
                      onSelected: (selected) {
                        provider.setPriorityFilter(selected ? p : null);
                        setSheetState(() {});
                      },
                    );
                  }).toList(),
                ),
                const SizedBox(height: AppSpacing.lg),
                Text('Risk Level', style: Theme.of(context).textTheme.titleSmall),
                const SizedBox(height: AppSpacing.sm),
                Wrap(
                  spacing: AppSpacing.sm,
                  runSpacing: AppSpacing.sm,
                  children: RiskLevel.values.map((r) {
                    return AppFilterChip(
                      label: r.label,
                      selected: provider.riskFilter == r,
                      onSelected: (selected) {
                        provider.setRiskFilter(selected ? r : null);
                        setSheetState(() {});
                      },
                    );
                  }).toList(),
                ),
                const SizedBox(height: AppSpacing.lg),
                Text('Status', style: Theme.of(context).textTheme.titleSmall),
                const SizedBox(height: AppSpacing.sm),
                Wrap(
                  spacing: AppSpacing.sm,
                  runSpacing: AppSpacing.sm,
                  children: RecoveryStatus.values.map((s) {
                    return AppFilterChip(
                      label: s.label,
                      selected: provider.statusFilter == s,
                      onSelected: (selected) {
                        provider.setStatusFilter(selected ? s : null);
                        setSheetState(() {});
                      },
                    );
                  }).toList(),
                ),
                const SizedBox(height: AppSpacing.lg),
                Row(
                  children: [
                    Expanded(
                      child: AppButton(
                        label: 'Clear',
                        variant: AppButtonVariant.outline,
                        onPressed: () {
                          provider.clearFilters();
                          setSheetState(() {});
                        },
                      ),
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    Expanded(child: AppButton(label: 'Apply', onPressed: () => Navigator.of(context).pop())),
                  ],
                ),
              ],
            ),
          );
        },
      ),
    ).then((_) => setState(() {}));
  }

  @override
  Widget build(BuildContext context) {
    final provider = context.watch<RecoveryCasesProvider>();

    return AppScaffold(
      title: 'Recovery Cases',
      subtitle: '${provider.filtered.length} cases',
      padding: const EdgeInsets.all(AppSpacing.md),
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          AppSearchBar(
            hint: 'Search by name or loan ID',
            onChanged: provider.setSearchQuery,
            onFilterTap: _openFilters,
            hasActiveFilters: _hasActiveFilters,
          ),
          const SizedBox(height: AppSpacing.md),
          Expanded(
            child: provider.isLoading
                ? const Center(child: CircularProgressIndicator())
                : provider.errorMessage != null
                    ? Center(
                        child: Column(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Text(provider.errorMessage!, style: Theme.of(context).textTheme.bodyMedium, textAlign: TextAlign.center),
                            const SizedBox(height: AppSpacing.md),
                            AppButton(label: 'Retry', onPressed: provider.load),
                          ],
                        ),
                      )
                    : provider.filtered.isEmpty
                    ? Center(
                        child: Text('No cases match your filters', style: Theme.of(context).textTheme.bodyMedium),
                      )
                    : RefreshIndicator(
                        onRefresh: provider.load,
                        child: ListView.separated(
                          physics: const AlwaysScrollableScrollPhysics(),
                          itemCount: provider.filtered.length,
                          separatorBuilder: (_, __) => const SizedBox(height: AppSpacing.sm),
                          itemBuilder: (context, index) => _RecoveryCaseCard(caseItem: provider.filtered[index]),
                        ),
                      ),
          ),
        ],
      ),
    );
  }
}

class _RecoveryCaseCard extends StatelessWidget {
  const _RecoveryCaseCard({required this.caseItem});

  final RecoveryCaseEntity caseItem;

  AppBadgeVariant get _priorityVariant => switch (caseItem.priority) {
        RecoveryPriority.high => AppBadgeVariant.danger,
        RecoveryPriority.medium => AppBadgeVariant.warning,
        RecoveryPriority.low => AppBadgeVariant.info,
      };

  AppBadgeVariant get _riskVariant => switch (caseItem.riskLevel) {
        RiskLevel.low => AppBadgeVariant.success,
        RiskLevel.medium => AppBadgeVariant.warning,
        RiskLevel.high => AppBadgeVariant.danger,
        RiskLevel.critical => AppBadgeVariant.danger,
      };

  AppBadgeVariant get _statusVariant => switch (caseItem.status) {
        RecoveryStatus.assigned => AppBadgeVariant.info,
        RecoveryStatus.contacted => AppBadgeVariant.info,
        RecoveryStatus.visitScheduled => AppBadgeVariant.warning,
        RecoveryStatus.visited => AppBadgeVariant.warning,
        RecoveryStatus.promiseToPay => AppBadgeVariant.gold,
        RecoveryStatus.partialPayment => AppBadgeVariant.gold,
        RecoveryStatus.fullPayment => AppBadgeVariant.success,
        RecoveryStatus.settlement => AppBadgeVariant.warning,
        RecoveryStatus.escalated => AppBadgeVariant.danger,
        RecoveryStatus.legalRecommended => AppBadgeVariant.danger,
        RecoveryStatus.unableToLocate => AppBadgeVariant.neutral,
        RecoveryStatus.fraudSuspicion => AppBadgeVariant.danger,
        RecoveryStatus.closed => AppBadgeVariant.success,
      };

  @override
  Widget build(BuildContext context) {
    return AppCard(
      onTap: () => context.push(
        RouteNames.recoveryCaseDetailPath.replaceFirst(':id', caseItem.id),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(caseItem.customerName, style: Theme.of(context).textTheme.titleSmall, maxLines: 1, overflow: TextOverflow.ellipsis),
              ),
              AppBadge(label: caseItem.priority.label, variant: _priorityVariant),
            ],
          ),
          const SizedBox(height: 4),
          Text('${caseItem.loanId} · ${caseItem.loanType.label}',
              style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary)),
          const SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              const Icon(Icons.currency_rupee, size: 14, color: AppColors.textSecondary),
              Flexible(child: Text(caseItem.outstandingAmount.toStringAsFixed(0), style: Theme.of(context).textTheme.bodySmall)),
              const SizedBox(width: AppSpacing.md),
              const Icon(Icons.event_busy_outlined, size: 14, color: AppColors.textSecondary),
              const SizedBox(width: 2),
              Text('${caseItem.dpd} DPD', style: Theme.of(context).textTheme.bodySmall),
              const SizedBox(width: AppSpacing.md),
              const Icon(Icons.receipt_long_outlined, size: 14, color: AppColors.textSecondary),
              const SizedBox(width: 2),
              Text('${caseItem.overdueEmiCount} EMIs', style: Theme.of(context).textTheme.bodySmall),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              const Icon(Icons.apartment_outlined, size: 14, color: AppColors.textSecondary),
              const SizedBox(width: 4),
              Expanded(child: Text(caseItem.branch, style: Theme.of(context).textTheme.bodySmall, maxLines: 1, overflow: TextOverflow.ellipsis)),
              Text(
                'Assigned ${caseItem.assignedDate.day}/${caseItem.assignedDate.month}',
                style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          Wrap(
            spacing: AppSpacing.sm,
            runSpacing: 4,
            children: [
              AppBadge(label: caseItem.riskLevel.label, variant: _riskVariant),
              AppBadge(label: caseItem.status.label, variant: _statusVariant),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              Expanded(
                child: AppButton(
                  label: 'Open',
                  size: AppButtonSize.small,
                  icon: Icons.arrow_forward,
                  onPressed: () => context.push(RouteNames.recoveryCaseDetailPath.replaceFirst(':id', caseItem.id)),
                ),
              ),
              const SizedBox(width: AppSpacing.sm),
              Expanded(
                child: AppButton(
                  label: 'Call',
                  size: AppButtonSize.small,
                  icon: Icons.call_outlined,
                  variant: AppButtonVariant.outline,
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
                  size: AppButtonSize.small,
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
