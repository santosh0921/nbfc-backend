import 'package:flutter/material.dart';
import '../theme/app_colors.dart';
import '../theme/app_radius.dart';

/// A full-screen, impossible-to-miss takeover for a mandatory
/// notification (Sanction Letter / Disbursement Letter ready to sign) —
/// previously these only surfaced as a badge + a bell-shake animation on
/// the home screen, easy to notice and dismiss without ever acting on.
/// This forces the customer to explicitly choose to review it now or
/// defer, rather than letting a time-sensitive action slip by unseen.
class BigNotificationPage extends StatelessWidget {
  const BigNotificationPage({
    super.key,
    required this.letterType,
    required this.body,
    required this.onReviewNow,
  });

  final String letterType; // "Sanction Letter" | "Disbursement Letter"
  final String body;
  final VoidCallback onReviewNow;

  @override
  Widget build(BuildContext context) {
    final isDisbursement = letterType == 'Disbursement Letter';
    return Scaffold(
      backgroundColor: AppColors.primary,
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(28, 20, 28, 32),
          child: Column(
            children: [
              Align(
                alignment: Alignment.topRight,
                child: IconButton(
                  onPressed: () => Navigator.of(context).maybePop(),
                  icon: const Icon(Icons.close_rounded, color: Colors.white),
                  tooltip: 'Remind me later',
                ),
              ),
              const Spacer(),
              TweenAnimationBuilder<double>(
                tween: Tween(begin: 0.6, end: 1),
                duration: const Duration(milliseconds: 500),
                curve: Curves.elasticOut,
                builder: (context, scale, child) =>
                    Transform.scale(scale: scale, child: child),
                child: Container(
                  width: 96,
                  height: 96,
                  alignment: Alignment.center,
                  decoration: BoxDecoration(
                    color: Colors.white.withValues(alpha: 0.16),
                    shape: BoxShape.circle,
                  ),
                  child: Icon(
                    isDisbursement
                        ? Icons.account_balance_wallet_rounded
                        : Icons.description_rounded,
                    color: Colors.white,
                    size: 46,
                  ),
                ),
              ),
              const SizedBox(height: 28),
              Text(
                isDisbursement
                    ? 'Your Loan Has Been Disbursed!'
                    : 'Your Loan Has Been Sanctioned!',
                textAlign: TextAlign.center,
                style: const TextStyle(
                    color: Colors.white,
                    fontSize: 26,
                    fontWeight: FontWeight.w800,
                    height: 1.25),
              ),
              const SizedBox(height: 14),
              Text(
                body,
                textAlign: TextAlign.center,
                style: TextStyle(
                    color: Colors.white.withValues(alpha: 0.85),
                    fontSize: 15,
                    height: 1.5),
              ),
              const SizedBox(height: 20),
              Container(
                padding:
                    const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
                decoration: BoxDecoration(
                  color: Colors.white.withValues(alpha: 0.14),
                  borderRadius: BorderRadius.circular(AppRadius.pill),
                  border:
                      Border.all(color: Colors.white.withValues(alpha: 0.24)),
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Icon(Icons.edit_document,
                        color: Colors.white, size: 16),
                    const SizedBox(width: 8),
                    Text(
                      'Your signature is required on the $letterType',
                      style: const TextStyle(
                          color: Colors.white,
                          fontSize: 12.5,
                          fontWeight: FontWeight.w600),
                    ),
                  ],
                ),
              ),
              const Spacer(flex: 2),
              SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  onPressed: onReviewNow,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: Colors.white,
                    foregroundColor: AppColors.primary,
                    minimumSize: const Size.fromHeight(54),
                    shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(AppRadius.md)),
                  ),
                  child: const Text('Review & Sign Now',
                      style:
                          TextStyle(fontWeight: FontWeight.w700, fontSize: 16)),
                ),
              ),
              const SizedBox(height: 12),
              TextButton(
                onPressed: () => Navigator.of(context).maybePop(),
                child: const Text('Remind me later',
                    style: TextStyle(color: Colors.white70)),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
