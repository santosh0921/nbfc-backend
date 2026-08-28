import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../../core/network/api_exception.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/theme/app_colors.dart';
import 'widgets/auth_illustration.dart';
import 'widgets/code_boxes.dart';

/// Returning-user login: reached right after OTP verification when the
/// backend reports the mobile already has an account
/// ([AuthProvider.pendingAccountExists]). Asks for the existing MPIN and
/// logs in with it — deliberately never routes through Create MPIN, which
/// would silently overwrite the customer's real MPIN with whatever they
/// typed next.
class LoginMpinPage extends StatefulWidget {
  const LoginMpinPage({super.key, required this.mobile});

  final String mobile;

  @override
  State<LoginMpinPage> createState() => _LoginMpinPageState();
}

class _LoginMpinPageState extends State<LoginMpinPage> {
  String _mpin = '';
  bool _isSubmitting = false;
  bool _obscure = true;
  String? _error;

  bool get _canSubmit => _mpin.length == 6;

  Future<void> _submit() async {
    if (!_canSubmit || _isSubmitting) return;
    setState(() {
      _isSubmitting = true;
      _error = null;
    });
    try {
      await AuthProvider.instance.loginWithMpin(widget.mobile, _mpin);
      if (!mounted) return;
      context.go('/');
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _isSubmitting = false;
        _mpin = '';
        _error = e.message;
      });
    }
  }

  Future<void> _forgotMpin() async {
    context.push('/forgot-mpin', extra: widget.mobile);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(24, 8, 24, 24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const AuthIllustration(scene: AuthScene.mpin),
              const SizedBox(height: 24),
              Text('Welcome Back', textAlign: TextAlign.center, style: theme.textTheme.headlineMedium),
              const SizedBox(height: 8),
              Text(
                'Enter your MPIN to continue as +91 ${widget.mobile}',
                textAlign: TextAlign.center,
                style: theme.textTheme.bodyMedium,
              ),
              const SizedBox(height: 32),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text('MPIN', style: theme.textTheme.labelMedium),
                  InkWell(
                    onTap: () => setState(() => _obscure = !_obscure),
                    borderRadius: BorderRadius.circular(20),
                    child: Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(_obscure ? Icons.visibility_off_rounded : Icons.visibility_rounded, size: 16, color: AppColors.primary),
                          const SizedBox(width: 4),
                          Text(
                            _obscure ? 'Show' : 'Hide',
                            style: theme.textTheme.labelSmall?.copyWith(color: AppColors.primary, fontWeight: FontWeight.w700),
                          ),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 10),
              CodeBoxes(
                obscure: _obscure,
                autofocus: true,
                hasError: _error != null,
                onChanged: (v) => setState(() {
                  _mpin = v;
                  _error = null;
                }),
                onCompleted: (v) => setState(() => _mpin = v),
              ),
              if (_error != null) ...[
                const SizedBox(height: 10),
                Text(_error!, textAlign: TextAlign.center, style: theme.textTheme.bodySmall?.copyWith(color: AppColors.error)),
              ],
              const SizedBox(height: 10),
              Center(
                child: TextButton(
                  onPressed: _isSubmitting ? null : _forgotMpin,
                  child: const Text('Forgot MPIN?'),
                ),
              ),
              const SizedBox(height: 14),
              ElevatedButton(
                onPressed: _canSubmit && !_isSubmitting ? _submit : null,
                style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(54)),
                child: _isSubmitting
                    ? const SizedBox(
                        width: 22,
                        height: 22,
                        child: CircularProgressIndicator(strokeWidth: 2.4, valueColor: AlwaysStoppedAnimation(Colors.white)),
                      )
                    : const Text('Login'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
