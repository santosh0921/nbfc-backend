import 'package:flutter/material.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';
import '../../apply/widgets/apply_form_field.dart';
import '../../auth/widgets/code_boxes.dart';

/// Mimics the redirect-to-bank step of a real net banking payment: the
/// user is "sent" to their bank's login page, authenticates, then
/// authorizes the transaction with an OTP — rather than the old flow
/// where picking a bank and tapping Pay just succeeded instantly.
/// Pops `true` if the user completes authorization, `false`/null if they
/// back out at any point.
class NetBankingAuthScreen extends StatefulWidget {
  const NetBankingAuthScreen({super.key, required this.bankName, required this.amountLabel});

  final String bankName;
  final String amountLabel;

  @override
  State<NetBankingAuthScreen> createState() => _NetBankingAuthScreenState();
}

enum _Stage { login, authenticating, otp, authorizing }

class _NetBankingAuthScreenState extends State<NetBankingAuthScreen> {
  _Stage _stage = _Stage.login;
  final _userIdController = TextEditingController();
  final _passwordController = TextEditingController();
  String _otp = '';
  bool _obscure = true;

  @override
  void dispose() {
    _userIdController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  bool get _canLogin => _userIdController.text.trim().isNotEmpty && _passwordController.text.trim().length >= 4;

  Future<void> _login() async {
    if (!_canLogin) return;
    setState(() => _stage = _Stage.authenticating);
    await Future<void>.delayed(const Duration(milliseconds: 1100));
    if (!mounted) return;
    setState(() => _stage = _Stage.otp);
  }

  Future<void> _authorize() async {
    if (_otp.length != 6) return;
    setState(() => _stage = _Stage.authorizing);
    await Future<void>.delayed(const Duration(milliseconds: 1200));
    if (!mounted) return;
    Navigator.of(context).pop(true);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isBusy = _stage == _Stage.authenticating || _stage == _Stage.authorizing;

    return Scaffold(
      backgroundColor: AppColors.backgroundLight,
      appBar: AppBar(
        title: Text(widget.bankName),
        leading: IconButton(
          icon: const Icon(Icons.close_rounded),
          onPressed: isBusy ? null : () => Navigator.of(context).pop(false),
        ),
      ),
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Container(
                  padding: const EdgeInsets.all(16),
                  decoration: BoxDecoration(
                    color: AppColors.primary.withValues(alpha: 0.08),
                    borderRadius: BorderRadius.circular(AppRadius.lg),
                  ),
                  child: Row(
                    children: [
                      Container(
                        width: 44,
                        height: 44,
                        decoration: BoxDecoration(color: AppColors.primary, borderRadius: BorderRadius.circular(AppRadius.sm)),
                        child: const Icon(Icons.account_balance_rounded, color: Colors.white),
                      ),
                      const SizedBox(width: 14),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(widget.bankName, style: theme.textTheme.titleSmall),
                            Text('Secure net banking redirect', style: theme.textTheme.bodySmall),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 24),
                if (_stage == _Stage.login || _stage == _Stage.authenticating) ...[
                  Text('Log in to authorize your payment of ${widget.amountLabel}', style: theme.textTheme.bodyMedium, textAlign: TextAlign.center),
                  const SizedBox(height: 24),
                  ApplyFormField(label: 'User ID', controller: _userIdController, hint: 'Enter your net banking user ID', onChanged: (_) => setState(() {})),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text('Password', style: theme.textTheme.titleSmall),
                      InkWell(
                        onTap: () => setState(() => _obscure = !_obscure),
                        child: Icon(_obscure ? Icons.visibility_off_rounded : Icons.visibility_rounded, size: 18, color: AppColors.primary),
                      ),
                    ],
                  ),
                  const SizedBox(height: 8),
                  ApplyFormField(
                    label: '',
                    controller: _passwordController,
                    hint: 'Enter your password',
                    obscureText: _obscure,
                    onChanged: (_) => setState(() {}),
                  ),
                  const SizedBox(height: 16),
                  ElevatedButton(
                    onPressed: _canLogin && !isBusy ? _login : null,
                    style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(52)),
                    child: _stage == _Stage.authenticating
                        ? const SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(strokeWidth: 2.2, valueColor: AlwaysStoppedAnimation(Colors.white)),
                          )
                        : const Text('Login'),
                  ),
                ] else ...[
                  const Icon(Icons.verified_user_rounded, size: 40, color: AppColors.primary),
                  const SizedBox(height: 12),
                  Text(
                    'Enter the OTP sent to your registered mobile to authorize this payment of ${widget.amountLabel}',
                    style: theme.textTheme.bodyMedium,
                    textAlign: TextAlign.center,
                  ),
                  const SizedBox(height: 24),
                  CodeBoxes(autofocus: true, onChanged: (v) => setState(() => _otp = v)),
                  const SizedBox(height: 24),
                  ElevatedButton(
                    onPressed: _otp.length == 6 && !isBusy ? _authorize : null,
                    style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(52)),
                    child: _stage == _Stage.authorizing
                        ? const SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(strokeWidth: 2.2, valueColor: AlwaysStoppedAnimation(Colors.white)),
                          )
                        : const Text('Authorize Payment'),
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }
}
