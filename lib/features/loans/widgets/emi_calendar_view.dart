import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';
import '../../../core/widgets/premium_card.dart';
import '../../../models/emi_schedule.dart';

/// A month-grid calendar for an EMI schedule — each day carrying an
/// installment due date is highlighted by that installment's status
/// (paid/upcoming/overdue), with month navigation. Pure Flutter, no
/// external calendar package: the schedule only ever has one due date
/// per month, so a full-featured calendar library would be overkill for
/// what's really just "highlight up to one cell per month".
class EmiCalendarView extends StatefulWidget {
  const EmiCalendarView({super.key, required this.installments, required this.onSelectInstallment});

  final List<EmiInstallment> installments;
  final ValueChanged<EmiInstallment> onSelectInstallment;

  @override
  State<EmiCalendarView> createState() => _EmiCalendarViewState();
}

class _EmiCalendarViewState extends State<EmiCalendarView> {
  late DateTime _visibleMonth;

  static const _weekdayLabels = ['M', 'T', 'W', 'T', 'F', 'S', 'S'];

  @override
  void initState() {
    super.initState();
    final firstUnpaid = widget.installments.where((i) => i.status != 'paid').toList()
      ..sort((a, b) => a.dueDate.compareTo(b.dueDate));
    final anchor = firstUnpaid.isNotEmpty ? firstUnpaid.first.dueDate : (widget.installments.isNotEmpty ? widget.installments.first.dueDate : DateTime.now());
    _visibleMonth = DateTime(anchor.year, anchor.month);
  }

  Map<DateTime, EmiInstallment> get _byDate => {
        for (final i in widget.installments) DateTime(i.dueDate.year, i.dueDate.month, i.dueDate.day): i,
      };

  Color _colorFor(String status) => switch (status) {
        'paid' => AppColors.success,
        'overdue' => AppColors.error,
        _ => AppColors.primary,
      };

  void _changeMonth(int delta) {
    setState(() => _visibleMonth = DateTime(_visibleMonth.year, _visibleMonth.month + delta));
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final byDate = _byDate;
    final firstOfMonth = DateTime(_visibleMonth.year, _visibleMonth.month, 1);
    final daysInMonth = DateTime(_visibleMonth.year, _visibleMonth.month + 1, 0).day;
    // DateTime.weekday: Mon=1..Sun=7 — leading blanks before the 1st.
    final leadingBlanks = firstOfMonth.weekday - 1;

    return PremiumCard(
      child: Column(
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              IconButton(icon: const Icon(Icons.chevron_left_rounded), onPressed: () => _changeMonth(-1)),
              Text(DateFormat('MMMM yyyy').format(_visibleMonth), style: theme.textTheme.titleSmall),
              IconButton(icon: const Icon(Icons.chevron_right_rounded), onPressed: () => _changeMonth(1)),
            ],
          ),
          const SizedBox(height: 4),
          Row(
            children: [
              for (final w in _weekdayLabels)
                Expanded(
                  child: Center(
                    child: Text(w, style: theme.textTheme.labelSmall?.copyWith(color: AppColors.textSecondaryLight)),
                  ),
                ),
            ],
          ),
          const SizedBox(height: 4),
          GridView.builder(
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(crossAxisCount: 7),
            itemCount: leadingBlanks + daysInMonth,
            itemBuilder: (context, index) {
              if (index < leadingBlanks) return const SizedBox.shrink();
              final day = index - leadingBlanks + 1;
              final date = DateTime(_visibleMonth.year, _visibleMonth.month, day);
              final installment = byDate[date];
              final color = installment == null ? null : _colorFor(installment.status);

              return Padding(
                padding: const EdgeInsets.all(2),
                child: InkWell(
                  borderRadius: BorderRadius.circular(AppRadius.sm),
                  onTap: installment == null ? null : () => widget.onSelectInstallment(installment),
                  child: Container(
                    alignment: Alignment.center,
                    decoration: color == null
                        ? null
                        : BoxDecoration(color: color.withValues(alpha: 0.15), shape: BoxShape.circle, border: Border.all(color: color, width: 1.4)),
                    child: Text(
                      '$day',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: color ?? theme.textTheme.bodyMedium?.color,
                        fontWeight: color == null ? FontWeight.normal : FontWeight.w700,
                      ),
                    ),
                  ),
                ),
              );
            },
          ),
          const SizedBox(height: 8),
          Wrap(
            spacing: 14,
            runSpacing: 4,
            children: [
              _LegendDot(color: AppColors.success, label: 'Paid'),
              _LegendDot(color: AppColors.primary, label: 'Upcoming'),
              _LegendDot(color: AppColors.error, label: 'Overdue'),
            ],
          ),
        ],
      ),
    );
  }
}

class _LegendDot extends StatelessWidget {
  const _LegendDot({required this.color, required this.label});

  final Color color;
  final String label;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(width: 10, height: 10, decoration: BoxDecoration(color: color, shape: BoxShape.circle)),
        const SizedBox(width: 6),
        Text(label, style: Theme.of(context).textTheme.labelSmall),
      ],
    );
  }
}
