import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../../core/network/api_exception.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import 'widgets/auth_illustration.dart';

/// Mobile-number entry screen — the first step of the login flow.
/// Requests a real OTP from the backend via [AuthProvider.requestOtp].
class LoginPage extends StatefulWidget {
  const LoginPage({super.key});

  @override
  State<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends State<LoginPage> {
  final TextEditingController _phoneController = TextEditingController();
  bool _isSubmitting = false;

  bool get _isValid => _phoneController.text.trim().length == 10;

  @override
  void dispose() {
    _phoneController.dispose();
    super.dispose();
  }

  Future<void> _continue() async {
    if (!_isValid) return;
    setState(() => _isSubmitting = true);
    String? otp;
    String? error;
    try {
      otp = await AuthProvider.instance.requestOtp(_phoneController.text.trim());
    } on ApiException catch (e) {
      error = e.message;
    }
    if (!mounted) return;
    setState(() => _isSubmitting = false);
    if (error != null) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(error)));
      return;
    }
    context.push('/otp', extra: (phone: _phoneController.text.trim(), devOtp: otp));
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        centerTitle: true,
        title: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 26,
              height: 26,
              decoration: BoxDecoration(color: AppColors.primary, borderRadius: BorderRadius.circular(8)),
              child: const Icon(Icons.account_balance_rounded, color: Colors.white, size: 15),
            ),
            const SizedBox(width: 8),
            Text('NBFC Premium', style: theme.textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w800)),
          ],
        ),
      ),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
          child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      const AuthIllustration(scene: AuthScene.login),
                      const SizedBox(height: 28),
                      Text(
                        'Login',
                        textAlign: TextAlign.center,
                        style: theme.textTheme.headlineMedium,
                      ),
                      const SizedBox(height: 8),
                      Text(
                        'Login to continue using the NBFC Premium app.',
                        textAlign: TextAlign.center,
                        style: theme.textTheme.bodyMedium,
                      ),
                      const SizedBox(height: 28),
                      Text('Mobile Number', style: theme.textTheme.titleSmall),
                      const SizedBox(height: 10),
                      Container(
                        decoration: BoxDecoration(
                          color: AppColors.backgroundLight,
                          border: Border.all(color: AppColors.borderLight),
                          borderRadius: BorderRadius.circular(AppRadius.md),
                        ),
                        padding: const EdgeInsets.symmetric(horizontal: 16),
                        child: Row(
                          children: [
                            const Icon(Icons.call_rounded, size: 18, color: AppColors.primary),
                            const SizedBox(width: 10),
                            Text('+91', style: theme.textTheme.titleMedium),
                            const SizedBox(width: 10),
                            Container(width: 1, height: 24, color: AppColors.borderLight),
                            const SizedBox(width: 10),
                            Expanded(
                              child: TextField(
                                controller: _phoneController,
                                keyboardType: TextInputType.phone,
                                maxLength: 10,
                                style: theme.textTheme.titleMedium,
                                decoration: const InputDecoration(
                                  counterText: '',
                                  border: InputBorder.none,
                                  hintText: 'Enter here',
                                  contentPadding: EdgeInsets.symmetric(vertical: 16),
                                ),
                                onChanged: (_) => setState(() {}),
                              ),
                            ),
                          ],
                        ),
                      ),
                      const SizedBox(height: 12),
                      Center(
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Text('Having difficulties logging-in? ', style: theme.textTheme.bodySmall),
                            Text(
                              'Get Help',
                              style: theme.textTheme.bodySmall?.copyWith(color: AppColors.primary, fontWeight: FontWeight.w700),
                            ),
                          ],
                        ),
                      ),
                      const SizedBox(height: 28),
                      ElevatedButton(
                        onPressed: _isValid && !_isSubmitting ? _continue : null,
                        style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(54)),
                        child: _isSubmitting
                            ? const SizedBox(
                                width: 22,
                                height: 22,
                                child: CircularProgressIndicator(strokeWidth: 2.4, valueColor: AlwaysStoppedAnimation(Colors.white)),
                              )
                            : const Text('Continue'),
                      ),
                      const SizedBox(height: 32),
                      Center(
                        child: Text(
                          'By continuing, you agree to our Terms & Privacy Policy.',
                          textAlign: TextAlign.center,
                          style: theme.textTheme.labelSmall,
                        ),
                      ),
                      const SizedBox(height: 20),
                      Center(
                        child: TextButton.icon(
                          onPressed: () => context.push('/employee'),
                          icon: const Icon(Icons.badge_outlined, size: 18),
                          label: const Text('Login as Employee'),
                          style: TextButton.styleFrom(foregroundColor: AppColors.textSecondaryLight),
                        ),
                      ),
            ],
          ),
        ),
      ),
    );
  }
}
