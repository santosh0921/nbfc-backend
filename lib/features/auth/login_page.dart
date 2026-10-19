import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../../core/network/api_exception.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import '../../core/widgets/premium_button.dart';
import '../../core/widgets/premium_card.dart';
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
      extendBodyBehindAppBar: true,
      appBar: AppBar(
        backgroundColor: Colors.transparent,
        centerTitle: true,
        title: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 26,
              height: 26,
              decoration: BoxDecoration(
                gradient: const LinearGradient(colors: AppColors.gradientGold),
                borderRadius: BorderRadius.circular(8),
              ),
              child: const Icon(Icons.account_balance_rounded, color: AppColors.onPrimary, size: 15),
            ),
            const SizedBox(width: 8),
            Text('OneFinance', style: theme.textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w800)),
          ],
        ),
      ),
      // A minimal, static gold-to-cream backdrop — the "trusted financial
      // platform" first impression the design system calls for. Kept to a
      // plain gradient (no animated rays/blur) so the login screen, which
      // has to render instantly, never waits on anything heavier.
      body: DecoratedBox(
        decoration: BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [AppColors.primaryLight, AppColors.backgroundOf(context)],
            stops: const [0.0, 0.35],
          ),
        ),
        child: SafeArea(
          child: SingleChildScrollView(
            padding: EdgeInsets.fromLTRB(24, MediaQuery.of(context).padding.top > 0 ? 8 : 16, 24, 24),
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
                  'Login to continue using the OneFinance app.',
                  textAlign: TextAlign.center,
                  style: theme.textTheme.bodyMedium,
                ),
                const SizedBox(height: 28),
                PremiumCard(
                  shine: true,
                  padding: const EdgeInsets.all(20),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Text('Mobile Number', style: theme.textTheme.titleSmall),
                      const SizedBox(height: 10),
                      Container(
                        decoration: BoxDecoration(
                          color: theme.brightness == Brightness.dark ? AppColors.surfaceDark : Colors.white,
                          border: Border.all(color: AppColors.borderOf(context)),
                          borderRadius: BorderRadius.circular(AppRadius.sm),
                        ),
                        padding: const EdgeInsets.symmetric(horizontal: 16),
                        child: Row(
                          children: [
                            const Icon(Icons.call_rounded, size: 18, color: AppColors.primaryDark),
                            const SizedBox(width: 10),
                            Text('+91', style: theme.textTheme.titleMedium),
                            const SizedBox(width: 10),
                            Container(width: 1, height: 24, color: AppColors.borderOf(context)),
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
                              style: theme.textTheme.bodySmall
                                  ?.copyWith(color: AppColors.primaryDark, fontWeight: FontWeight.w700),
                            ),
                          ],
                        ),
                      ),
                      const SizedBox(height: 20),
                      PremiumButton(
                        label: 'Continue',
                        loading: _isSubmitting,
                        onPressed: _isValid && !_isSubmitting ? _continue : null,
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 24),
                Center(
                  child: Text(
                    'By continuing, you agree to our Terms & Privacy Policy.',
                    textAlign: TextAlign.center,
                    style: theme.textTheme.labelSmall,
                  ),
                ),
                const SizedBox(height: 12),
                Center(
                  child: TextButton.icon(
                    onPressed: () => context.push('/employee'),
                    icon: const Icon(Icons.badge_outlined, size: 18),
                    label: const Text('Login as Employee'),
                    style: TextButton.styleFrom(foregroundColor: AppColors.textSecondaryOf(context)),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
