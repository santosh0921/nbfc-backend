import 'package:flutter/material.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import '../../core/widgets/premium_card.dart';
import '../../core/widgets/slider_input_field.dart';
import '../../mock/mock_data.dart';
import 'widgets/credit_score_gauge.dart';

String _bandFor(int score) {
  if (score >= 750) return 'Excellent';
  if (score >= 700) return 'Good';
  if (score >= 650) return 'Fair';
  if (score >= 550) return 'Poor';
  return 'Very Poor';
}

class _ScoreFactor {
  final String label;
  final String rating;
  final IconData icon;
  final Color color;

  const _ScoreFactor({required this.label, required this.rating, required this.icon, required this.color});
}

/// Clean, minimal credit-health screen — big gauge up top, a simple
/// icon-list of contributing factors, and a compact score-history strip.
/// No busy progress bars or dense cards; generous spacing throughout.
class CreditScoreScreen extends StatefulWidget {
  const CreditScoreScreen({super.key});

  @override
  State<CreditScoreScreen> createState() => _CreditScoreScreenState();
}

class _CreditScoreScreenState extends State<CreditScoreScreen> with SingleTickerProviderStateMixin {
  late final AnimationController _liveDotController = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1000),
  )..repeat(reverse: true);

  int? _calculatedScore;

  int get _displayScore => _calculatedScore ?? MockData.creditScore;
  String get _displayBand => _calculatedScore != null ? _bandFor(_calculatedScore!) : MockData.creditScoreBand;

  Future<void> _openCalculator() async {
    final result = await showModalBottomSheet<int>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (context) => const _CibilCalculatorSheet(),
    );
    if (result != null) setState(() => _calculatedScore = result);
  }

  static const _factors = [
    _ScoreFactor(label: 'Payment History', rating: 'Excellent', icon: Icons.event_available_rounded, color: AppColors.success),
    _ScoreFactor(label: 'Credit Utilization', rating: 'Good', icon: Icons.pie_chart_rounded, color: AppColors.success),
    _ScoreFactor(label: 'Credit Age', rating: 'Excellent', icon: Icons.history_rounded, color: AppColors.success),
    _ScoreFactor(label: 'Credit Mix', rating: 'Fair', icon: Icons.category_rounded, color: AppColors.warning),
    _ScoreFactor(label: 'Recent Inquiries', rating: 'Good', icon: Icons.search_rounded, color: AppColors.success),
  ];

  static const _history = [
    ('Feb', 742),
    ('Mar', 751),
    ('Apr', 760),
    ('May', 768),
    ('Jun', 775),
    ('Jul', 785),
  ];

  @override
  void dispose() {
    _liveDotController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    return Scaffold(
      backgroundColor: isDark ? AppColors.backgroundDark : AppColors.backgroundLight,
      appBar: AppBar(title: const Text('Credit Health')),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(20, 8, 20, 32),
          children: [
            Center(
              child: Column(
                children: [
                  CreditScoreGauge(score: _displayScore, width: 260, height: 150, strokeWidth: 20, showLabels: true, live: true),
                  const SizedBox(height: 8),
                  Text(
                    '$_displayScore',
                    style: theme.textTheme.displayLarge?.copyWith(fontWeight: FontWeight.w800),
                  ),
                  Text('CIBIL Score · out of 900', style: theme.textTheme.bodyMedium),
                  const SizedBox(height: 12),
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 7),
                    decoration: BoxDecoration(
                      color: AppColors.success.withValues(alpha: 0.12),
                      borderRadius: BorderRadius.circular(AppRadius.pill),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const Icon(Icons.check_circle_rounded, size: 15, color: AppColors.success),
                        const SizedBox(width: 6),
                        Text(
                          _displayBand,
                          style: theme.textTheme.labelLarge?.copyWith(color: AppColors.success, fontWeight: FontWeight.w700),
                        ),
                      ],
                    ),
                  ),
                  if (_calculatedScore != null) ...[
                    const SizedBox(height: 6),
                    Text(
                      'Simulated score — not your actual CIBIL report.',
                      style: theme.textTheme.labelSmall?.copyWith(color: AppColors.warning),
                    ),
                  ],
                  const SizedBox(height: 10),
                  Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      AnimatedBuilder(
                        animation: _liveDotController,
                        builder: (context, _) => Container(
                          width: 7,
                          height: 7,
                          decoration: BoxDecoration(
                            color: AppColors.success.withValues(alpha: 0.4 + _liveDotController.value * 0.6),
                            shape: BoxShape.circle,
                          ),
                        ),
                      ),
                      const SizedBox(width: 6),
                      Text(
                        'Live monitoring active · updated today',
                        style: theme.textTheme.labelSmall?.copyWith(color: AppColors.textSecondaryLight),
                      ),
                    ],
                  ),
                ],
              ),
            ),
            const SizedBox(height: 20),
            OutlinedButton.icon(
              onPressed: _openCalculator,
              icon: const Icon(Icons.calculate_rounded, size: 18),
              label: const Text('Calculate My CIBIL Score'),
              style: OutlinedButton.styleFrom(minimumSize: const Size.fromHeight(50)),
            ),
            const SizedBox(height: 36),
            Text('Score History', style: theme.textTheme.titleMedium),
            const SizedBox(height: 12),
            SizedBox(
              height: 64,
              child: Row(
                children: [
                  for (final entry in _history) ...[
                    Expanded(child: _HistoryChip(month: entry.$1, score: entry.$2, isLatest: entry == _history.last)),
                  ],
                ],
              ),
            ),
            const SizedBox(height: 36),
            Text('What\'s Affecting Your Score', style: theme.textTheme.titleMedium),
            const SizedBox(height: 4),
            Text('Based on your credit report factors.', style: theme.textTheme.bodySmall),
            const SizedBox(height: 12),
            PremiumCard(
              padding: EdgeInsets.zero,
              child: Column(
                children: [
                  for (int i = 0; i < _factors.length; i++) ...[
                    _FactorRow(factor: _factors[i]),
                    if (i != _factors.length - 1) const Divider(height: 1, indent: 60),
                  ],
                ],
              ),
            ),
            const SizedBox(height: 20),
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Icon(Icons.lightbulb_rounded, color: AppColors.warning, size: 20),
                const SizedBox(width: 10),
                Expanded(
                  child: Text(
                    'Keep credit utilization below 30% and avoid multiple loan inquiries in a short period to improve your score further.',
                    style: theme.textTheme.bodySmall,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _HistoryChip extends StatelessWidget {
  const _HistoryChip({required this.month, required this.score, required this.isLatest});

  final String month;
  final int score;
  final bool isLatest;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      mainAxisAlignment: MainAxisAlignment.end,
      children: [
        Text(
          '$score',
          style: theme.textTheme.labelSmall?.copyWith(
            fontWeight: isLatest ? FontWeight.w800 : FontWeight.w500,
            color: isLatest ? AppColors.primary : AppColors.textSecondaryLight,
          ),
        ),
        const SizedBox(height: 6),
        Container(
          width: 8,
          height: 8,
          decoration: BoxDecoration(
            color: isLatest ? AppColors.primary : AppColors.borderLight,
            shape: BoxShape.circle,
          ),
        ),
        const SizedBox(height: 6),
        Text(month, style: theme.textTheme.labelSmall),
      ],
    );
  }
}

class _FactorRow extends StatelessWidget {
  const _FactorRow({required this.factor});

  final _ScoreFactor factor;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
      child: Row(
        children: [
          Container(
            width: 36,
            height: 36,
            decoration: BoxDecoration(
              color: factor.color.withValues(alpha: 0.12),
              shape: BoxShape.circle,
            ),
            child: Icon(factor.icon, size: 18, color: factor.color),
          ),
          const SizedBox(width: 14),
          Expanded(child: Text(factor.label, style: theme.textTheme.bodyMedium)),
          Text(
            factor.rating,
            style: theme.textTheme.labelLarge?.copyWith(color: factor.color, fontWeight: FontWeight.w700),
          ),
        ],
      ),
    );
  }
}

/// A simple weighted CIBIL estimator modeled loosely on the real factor
/// weights CIBIL publishes (payment history ~35%, utilization ~30%,
/// credit age ~15%, credit mix ~10%, recent inquiries ~10%). Purely a
/// local simulation — not a real bureau pull.
class _CibilCalculatorSheet extends StatefulWidget {
  const _CibilCalculatorSheet();

  @override
  State<_CibilCalculatorSheet> createState() => _CibilCalculatorSheetState();
}

class _CibilCalculatorSheetState extends State<_CibilCalculatorSheet> {
  double _paymentHistory = 90; // % of EMIs/bills paid on time
  double _utilization = 30; // % of credit limit used
  double _creditAge = 4; // years
  double _creditMix = 2; // number of distinct credit types
  double _inquiries = 2; // hard inquiries in the last 12 months

  int get _estimatedScore {
    final paymentScore = (_paymentHistory / 100) * 0.35;
    final utilizationScore = (1 - (_utilization / 100).clamp(0, 1)) * 0.30;
    final ageScore = (_creditAge / 10).clamp(0, 1) * 0.15;
    final mixScore = (_creditMix / 4).clamp(0, 1) * 0.10;
    final inquiryScore = (1 - (_inquiries / 10).clamp(0, 1)) * 0.10;
    final weighted = paymentScore + utilizationScore + ageScore + mixScore + inquiryScore;
    return (300 + weighted * 600).round().clamp(300, 900);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final score = _estimatedScore;

    return Padding(
      padding: EdgeInsets.only(bottom: MediaQuery.of(context).viewInsets.bottom),
      child: Container(
        padding: const EdgeInsets.fromLTRB(20, 12, 20, 24),
        decoration: BoxDecoration(
          color: Theme.of(context).brightness == Brightness.dark ? AppColors.surfaceDark : AppColors.surfaceLight,
          borderRadius: const BorderRadius.vertical(top: Radius.circular(AppRadius.xl)),
        ),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Center(
                child: Container(width: 40, height: 4, decoration: BoxDecoration(color: AppColors.borderLight, borderRadius: BorderRadius.circular(4))),
              ),
              const SizedBox(height: 16),
              Text('Calculate My CIBIL Score', style: theme.textTheme.titleLarge),
              const SizedBox(height: 4),
              Text('Adjust these to see how each factor affects your score.', style: theme.textTheme.bodySmall),
              const SizedBox(height: 16),
              Container(
                width: double.infinity,
                padding: const EdgeInsets.all(16),
                decoration: BoxDecoration(
                  color: AppColors.primary.withValues(alpha: 0.08),
                  borderRadius: BorderRadius.circular(AppRadius.lg),
                ),
                child: Column(
                  children: [
                    Text('Estimated Score', style: theme.textTheme.bodySmall),
                    const SizedBox(height: 4),
                    Text('$score', style: theme.textTheme.displayMedium?.copyWith(color: AppColors.primary, fontWeight: FontWeight.w800)),
                    Text(_bandFor(score), style: theme.textTheme.titleSmall?.copyWith(color: AppColors.primary)),
                  ],
                ),
              ),
              const SizedBox(height: 20),
              SliderInputField(
                label: 'On-Time Payment History',
                value: _paymentHistory,
                min: 0,
                max: 100,
                divisions: 100,
                suffix: ' %',
                onChanged: (v) => setState(() => _paymentHistory = v),
              ),
              SliderInputField(
                label: 'Credit Utilization',
                value: _utilization,
                min: 0,
                max: 100,
                divisions: 100,
                suffix: ' %',
                onChanged: (v) => setState(() => _utilization = v),
              ),
              SliderInputField(
                label: 'Credit Age',
                value: _creditAge,
                min: 0,
                max: 20,
                divisions: 20,
                suffix: ' yrs',
                onChanged: (v) => setState(() => _creditAge = v),
              ),
              SliderInputField(
                label: 'Credit Mix (loan/card types)',
                value: _creditMix,
                min: 0,
                max: 5,
                divisions: 5,
                onChanged: (v) => setState(() => _creditMix = v),
              ),
              SliderInputField(
                label: 'Hard Inquiries (last 12 months)',
                value: _inquiries,
                min: 0,
                max: 10,
                divisions: 10,
                onChanged: (v) => setState(() => _inquiries = v),
              ),
              const SizedBox(height: 12),
              ElevatedButton(
                onPressed: () => Navigator.of(context).pop(score),
                style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(52)),
                child: const Text('Apply to My Score'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
