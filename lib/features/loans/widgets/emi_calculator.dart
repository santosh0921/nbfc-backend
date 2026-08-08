import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'dart:math' as math;
import '../../../core/theme/app_colors.dart';
import '../../../core/widgets/premium_card.dart';
import '../../../core/widgets/slider_input_field.dart';

class EmiCalculator extends StatefulWidget {
  final double interestRate;

  const EmiCalculator({super.key, required this.interestRate});

  @override
  State<EmiCalculator> createState() => _EmiCalculatorState();
}

class _EmiCalculatorState extends State<EmiCalculator> {
  double _amount = 500000;
  double _tenureMonths = 36;

  double get _emi {
    final r = widget.interestRate / 12 / 100;
    final n = _tenureMonths;
    if (r == 0) return _amount / n;
    return _amount * r * math.pow(1 + r, n) / (math.pow(1 + r, n) - 1);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final currency = NumberFormat.currency(locale: 'en_IN', symbol: '₹', decimalDigits: 0);

    return PremiumCard(
      padding: const EdgeInsets.all(20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('EMI Calculator', style: theme.textTheme.titleMedium),
          const SizedBox(height: 12),
          SliderInputField(
            label: 'Loan Amount',
            value: _amount,
            min: 50000,
            max: 5000000,
            divisions: 99,
            fieldWidth: 130,
            onChanged: (v) => setState(() => _amount = v),
          ),
          SliderInputField(
            label: 'Tenure',
            value: _tenureMonths,
            min: 6,
            max: 84,
            divisions: 78,
            suffix: ' mo',
            onChanged: (v) => setState(() => _tenureMonths = v),
          ),
          const SizedBox(height: 8),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: AppColors.primary.withValues(alpha: 0.08),
              borderRadius: BorderRadius.circular(16),
            ),
            child: Column(
              children: [
                Text('Estimated Monthly EMI', style: theme.textTheme.bodySmall),
                const SizedBox(height: 4),
                Text(
                  currency.format(_emi),
                  style: theme.textTheme.headlineMedium?.copyWith(color: AppColors.primary),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
