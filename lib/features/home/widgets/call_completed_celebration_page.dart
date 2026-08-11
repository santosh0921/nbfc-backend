import 'dart:async';
import 'package:flutter/material.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/widgets/success_celebration.dart';

/// Shown right after a customer's video verification call ends
/// successfully — "there's nothing to indicate the task is done"
/// applies to calls too, not just form submissions. Auto-dismisses after
/// a few seconds, or on tap.
class CallCompletedCelebrationPage extends StatefulWidget {
  const CallCompletedCelebrationPage({super.key});

  @override
  State<CallCompletedCelebrationPage> createState() => _CallCompletedCelebrationPageState();
}

class _CallCompletedCelebrationPageState extends State<CallCompletedCelebrationPage> {
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _timer = Timer(const Duration(seconds: 3), () {
      if (mounted) Navigator.of(context).maybePop();
    });
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      backgroundColor: AppColors.backgroundLight,
      body: SafeArea(
        child: GestureDetector(
          onTap: () => Navigator.of(context).maybePop(),
          behavior: HitTestBehavior.opaque,
          child: Center(
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const SuccessCelebration(size: 110),
                  const SizedBox(height: 20),
                  Text('One step closer!', style: theme.textTheme.headlineSmall, textAlign: TextAlign.center),
                  const SizedBox(height: 8),
                  Text(
                    "Your video verification call is complete. We'll keep you posted on the next step.",
                    textAlign: TextAlign.center,
                    style: theme.textTheme.bodyMedium?.copyWith(color: AppColors.textSecondaryLight),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
