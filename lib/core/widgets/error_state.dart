import 'package:flutter/material.dart';
import '../theme/app_colors.dart';
import 'premium_button.dart';

/// Consistent "something went wrong" surface with a retry action — never
/// show a raw exception/API error string to the user; pass a short, plain-
/// language [message] instead.
class ErrorState extends StatelessWidget {
  const ErrorState({
    super.key,
    this.title = 'Something went wrong',
    this.message = 'Please try again in a moment.',
    this.onRetry,
  });

  final String title;
  final String message;
  final VoidCallback? onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 48),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 64,
              height: 64,
              decoration: BoxDecoration(color: AppColors.error.withValues(alpha: 0.1), shape: BoxShape.circle),
              child: const Icon(Icons.error_outline_rounded, size: 30, color: AppColors.error),
            ),
            const SizedBox(height: 20),
            Text(title, style: Theme.of(context).textTheme.titleMedium, textAlign: TextAlign.center),
            const SizedBox(height: 6),
            Text(
              message,
              style: TextStyle(color: AppColors.textSecondaryOf(context), fontSize: 13.5),
              textAlign: TextAlign.center,
            ),
            if (onRetry != null) ...[
              const SizedBox(height: 20),
              PremiumButton(label: 'Try Again', onPressed: onRetry, expand: false),
            ],
          ],
        ),
      ),
    );
  }
}
