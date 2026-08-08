import 'dart:math' as math;
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import '../../core/widgets/premium_card.dart';
import '../../core/widgets/slider_input_field.dart';

/// Standalone EMI Calculator reachable from the Menu and Home screens —
/// works for any loan, not tied to a specific product. Lets the user
/// adjust amount, interest rate, and tenure, and shows a full principal
/// vs. interest breakdown.
class EmiCalculatorScreen extends StatefulWidget {
  const EmiCalculatorScreen({super.key});

  @override
  State<EmiCalculatorScreen> createState() => _EmiCalculatorScreenState();
}

class _EmiCalculatorScreenState extends State<EmiCalculatorScreen> {
  double _amount = 500000;
  double _rate = 10.5;
  double _tenureMonths = 36;

  double get _emi {
    final r = _rate / 12 / 100;
    final n = _tenureMonths;
    if (r == 0) return _amount / n;
    final factor = math.pow(1 + r, n);
    return _amount * r * factor / (factor - 1);
  }

  double get _totalPayment => _emi * _tenureMonths;
  double get _totalInterest => _totalPayment - _amount;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final currency = NumberFormat.currency(locale: 'en_IN', symbol: '₹', decimalDigits: 0);
    final principalShare = _amount / _totalPayment;

    return Scaffold(
      appBar: AppBar(title: const Text('EMI Calculator')),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(20, 8, 20, 32),
          children: [
            PremiumCard(
              padding: const EdgeInsets.all(20),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SliderInputField(
                    label: 'Loan Amount',
                    value: _amount,
                    min: 10000,
                    max: 5000000,
                    divisions: 499,
                    fieldWidth: 130,
                    onChanged: (v) => setState(() => _amount = v),
                  ),
                  SliderInputField(
                    label: 'Interest Rate',
                    value: _rate,
                    min: 5,
                    max: 24,
                    divisions: 190,
                    suffix: ' %',
                    fieldWidth: 90,
                    onChanged: (v) => setState(() => _rate = v),
                  ),
                  SliderInputField(
                    label: 'Tenure',
                    value: _tenureMonths,
                    min: 3,
                    max: 84,
                    divisions: 81,
                    suffix: ' mo',
                    fieldWidth: 90,
                    onChanged: (v) => setState(() => _tenureMonths = v),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 20),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(20),
              decoration: BoxDecoration(
                gradient: const LinearGradient(colors: AppColors.gradientBluePrimary, begin: Alignment.topLeft, end: Alignment.bottomRight),
                borderRadius: BorderRadius.circular(AppRadius.lg),
              ),
              child: Column(
                children: [
                  Text('Estimated Monthly EMI', style: theme.textTheme.bodyMedium?.copyWith(color: Colors.white.withValues(alpha: 0.85))),
                  const SizedBox(height: 6),
                  Text(
                    currency.format(_emi),
                    style: theme.textTheme.displaySmall?.copyWith(color: Colors.white, fontWeight: FontWeight.w800),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 20),
            PremiumCard(
              child: Column(
                children: [
                  ClipRRect(
                    borderRadius: BorderRadius.circular(AppRadius.pill),
                    child: SizedBox(
                      height: 10,
                      child: Row(
                        children: [
                          Expanded(flex: (principalShare * 1000).round().clamp(1, 999), child: Container(color: AppColors.primary)),
                          Expanded(flex: ((1 - principalShare) * 1000).round().clamp(1, 999), child: Container(color: AppColors.secondary)),
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(height: 16),
                  Row(
                    children: [
                      Expanded(
                        child: _BreakdownItem(color: AppColors.primary, label: 'Principal Amount', value: currency.format(_amount)),
                      ),
                      Expanded(
                        child: _BreakdownItem(color: AppColors.secondary, label: 'Total Interest', value: currency.format(_totalInterest)),
                      ),
                    ],
                  ),
                  const Divider(height: 28),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text('Total Payment', style: theme.textTheme.titleSmall),
                      Text(currency.format(_totalPayment), style: theme.textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w800)),
                    ],
                  ),
                ],
              ),
            ),
            const SizedBox(height: 20),
            OutlinedButton.icon(
              onPressed: () => context.push('/quick-apply'),
              icon: const Icon(Icons.arrow_forward_rounded, size: 18),
              label: const Text('Apply for This Loan'),
              style: OutlinedButton.styleFrom(minimumSize: const Size.fromHeight(52)),
            ),
          ],
        ),
      ),
    );
  }
}

class _BreakdownItem extends StatelessWidget {
  const _BreakdownItem({required this.color, required this.label, required this.value});

  final Color color;
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Container(width: 9, height: 9, decoration: BoxDecoration(color: color, shape: BoxShape.circle)),
            const SizedBox(width: 6),
            Expanded(child: Text(label, style: theme.textTheme.bodySmall, overflow: TextOverflow.ellipsis)),
          ],
        ),
        const SizedBox(height: 4),
        Text(value, style: theme.textTheme.titleSmall?.copyWith(fontWeight: FontWeight.w700)),
      ],
    );
  }
}
