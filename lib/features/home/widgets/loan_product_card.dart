import 'package:flutter/material.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';
import '../../../core/widgets/press_scale.dart';
import '../../../core/widgets/premium_card.dart';
import '../../../models/loan_product.dart';

class LoanProductCard extends StatelessWidget {
  final LoanProduct product;
  final VoidCallback onTap;

  const LoanProductCard({super.key, required this.product, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isPopular = product.category == LoanCategory.instantLoan;
    return PressScale(
      onTap: onTap,
      child: PremiumCard(
        padding: const EdgeInsets.all(14),
        child: Stack(
          children: [
            Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Container(
                  width: 44,
                  height: 44,
                  decoration: BoxDecoration(
                    gradient: LinearGradient(colors: product.gradient),
                    borderRadius: BorderRadius.circular(AppRadius.sm),
                  ),
                  child: Icon(product.icon, color: Colors.white, size: 22),
                ),
                const SizedBox(height: 10),
                Text(product.name, style: theme.textTheme.titleSmall, maxLines: 2, overflow: TextOverflow.ellipsis),
              ],
            ),
            if (isPopular)
              Positioned(
                top: 0,
                right: 0,
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 3),
                  decoration: BoxDecoration(
                    color: AppColors.success,
                    borderRadius: BorderRadius.circular(AppRadius.xs),
                  ),
                  child: const Text(
                    'POPULAR',
                    style: TextStyle(color: Colors.white, fontSize: 8, fontWeight: FontWeight.w800, letterSpacing: 0.2),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
