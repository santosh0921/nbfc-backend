import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../../../../core/components/app_bottom_sheet.dart';
import '../../../../core/components/app_card.dart';
import '../../../../core/components/app_filter_chips.dart';
import '../../../../core/components/app_scaffold.dart';
import '../../../../core/components/app_search_bar.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/widgets/app_badge.dart';
import '../../../../core/widgets/app_button.dart';
import '../../domain/entities/case_entity.dart';
import '../providers/cases_provider.dart';
import 'case_detail_page.dart';

class CasesListPage extends StatefulWidget {
  const CasesListPage({super.key});

  @override
  State<CasesListPage> createState() => _CasesListPageState();
}

class _CasesListPageState extends State<CasesListPage> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => context.read<CasesProvider>().load());
  }

  bool get _hasActiveFilters {
    final p = context.read<CasesProvider>();
    return p.loanTypeFilter != null || p.priorityFilter != null || p.statusFilter != null;
  }

  void _openFilters() {
    final provider = context.read<CasesProvider>();
    AppBottomSheet.show(
      context,
      title: 'Filter Cases',
      child: StatefulBuilder(
        builder: (context, setSheetState) {
          return Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Loan Type', style: Theme.of(context).textTheme.titleSmall),
              const SizedBox(height: AppSpacing.sm),
              Wrap(
                spacing: AppSpacing.sm,
                runSpacing: AppSpacing.sm,
                children: LoanType.values.map((type) {
                  return AppFilterChip(
                    label: type.label,
                    selected: provider.loanTypeFilter == type,
                    onSelected: (selected) {
                      provider.setLoanTypeFilter(selected ? type : null);
                      setSheetState(() {});
                    },
                  );
                }).toList(),
              ),
              const SizedBox(height: AppSpacing.lg),
              Text('Priority', style: Theme.of(context).textTheme.titleSmall),
              const SizedBox(height: AppSpacing.sm),
              Wrap(
                spacing: AppSpacing.sm,
                children: CasePriority.values.map((p) {
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
              Text('Status', style: Theme.of(context).textTheme.titleSmall),
              const SizedBox(height: AppSpacing.sm),
              Wrap(
                spacing: AppSpacing.sm,
                runSpacing: AppSpacing.sm,
                children: CaseStatus.values.map((s) {
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
          );
        },
      ),
    ).then((_) => setState(() {}));
  }

  @override
  Widget build(BuildContext context) {
    final provider = context.watch<CasesProvider>();

    return AppScaffold(
      title: 'Assigned Cases',
      subtitle: '${provider.filtered.length} cases',
      padding: const EdgeInsets.all(AppSpacing.md),
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          AppSearchBar(
            hint: 'Search by name or case number',
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
                          itemBuilder: (context, index) => _CaseCard(caseItem: provider.filtered[index]),
                        ),
                      ),
          ),
        ],
      ),
    );
  }
}

class _CaseCard extends StatelessWidget {
  const _CaseCard({required this.caseItem});

  final CaseEntity caseItem;

  AppBadgeVariant get _priorityVariant => switch (caseItem.priority) {
        CasePriority.high => AppBadgeVariant.danger,
        CasePriority.medium => AppBadgeVariant.warning,
        CasePriority.low => AppBadgeVariant.info,
      };

  AppBadgeVariant get _statusVariant => switch (caseItem.status) {
        CaseStatus.assigned => AppBadgeVariant.info,
        CaseStatus.inProgress => AppBadgeVariant.warning,
        CaseStatus.pendingReview => AppBadgeVariant.gold,
        CaseStatus.completed => AppBadgeVariant.success,
        CaseStatus.rejected => AppBadgeVariant.danger,
      };

  @override
  Widget build(BuildContext context) {
    return AppCard(
      onTap: () => Navigator.of(context).push(
        MaterialPageRoute(builder: (_) => CaseDetailPage(caseId: caseItem.id)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(caseItem.customerName, style: Theme.of(context).textTheme.titleSmall),
              ),
              AppBadge(label: caseItem.priority.label, variant: _priorityVariant),
            ],
          ),
          const SizedBox(height: 4),
          Text(caseItem.caseNumber, style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary)),
          const SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              Icon(Icons.account_balance_outlined, size: 14, color: AppColors.textSecondary),
              const SizedBox(width: 4),
              Text(caseItem.loanType.label, style: Theme.of(context).textTheme.bodySmall),
              const SizedBox(width: AppSpacing.md),
              Icon(Icons.currency_rupee, size: 14, color: AppColors.textSecondary),
              Text(caseItem.loanAmount.toStringAsFixed(0), style: Theme.of(context).textTheme.bodySmall),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              AppBadge(label: caseItem.status.label, variant: _statusVariant),
              Text(
                'Due ${caseItem.dueDate.day}/${caseItem.dueDate.month}',
                style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
