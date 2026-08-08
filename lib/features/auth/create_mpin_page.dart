import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../../core/network/api_exception.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/theme/app_colors.dart';
import 'widgets/auth_illustration.dart';
import 'widgets/code_boxes.dart';

/// Screen 3 of the login flow: sets a real 6-digit MPIN on the backend
/// (`/auth/create-mpin`), then logs in with it to obtain a JWT.
class CreateMpinPage extends StatefulWidget {
  const CreateMpinPage({super.key});

  @override
  State<CreateMpinPage> createState() => _CreateMpinPageState();
}

class _CreateMpinPageState extends State<CreateMpinPage> {
  String _mpin = '';
  String _confirmMpin = '';
  bool _isSubmitting = false;
  bool _obscure = true;

  bool get _hasMismatch => _confirmMpin.length == 6 && _confirmMpin != _mpin;
  bool get _isMatched => _confirmMpin.length == 6 && _confirmMpin == _mpin;
  bool get _canSubmit => _mpin.length == 6 && _isMatched;

  String? _error;

  Future<void> _continue() async {
    if (!_canSubmit) return;
    setState(() {
      _isSubmitting = true;
      _error = null;
    });
    try {
      await AuthProvider.instance.createMpinAndLogin(_mpin, _confirmMpin);
      if (!mounted) return;
      context.go('/biometric');
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _isSubmitting = false;
        _error = e.message;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(24, 24, 24, 24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const AuthIllustration(scene: AuthScene.mpin),
              const SizedBox(height: 24),
              Text('Create 6-Digit MPIN', textAlign: TextAlign.center, style: theme.textTheme.headlineMedium),
              const SizedBox(height: 8),
              Text(
                'Create a secure MPIN for quick access to your account.',
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
              CodeBoxes(obscure: _obscure, autofocus: true, onChanged: (v) => setState(() => _mpin = v)),
              const SizedBox(height: 24),
              Text('Confirm MPIN', style: theme.textTheme.labelMedium),
              const SizedBox(height: 10),
              CodeBoxes(obscure: _obscure, hasError: _hasMismatch, onChanged: (v) => setState(() => _confirmMpin = v)),
              const SizedBox(height: 10),
              if (_hasMismatch)
                Text('MPINs do not match.', style: theme.textTheme.bodySmall?.copyWith(color: AppColors.error))
              else if (_isMatched)
                Row(
                  mainAxisAlignment: MainAxisAlignment.start,
                  children: [
                    const Icon(Icons.check_circle_rounded, size: 16, color: AppColors.success),
                    const SizedBox(width: 6),
                    Text('MPINs match', style: theme.textTheme.bodySmall?.copyWith(color: AppColors.success)),
                  ],
                ),
              if (_error != null) ...[
                const SizedBox(height: 10),
                Text(_error!, textAlign: TextAlign.center, style: theme.textTheme.bodySmall?.copyWith(color: AppColors.error)),
              ],
              const SizedBox(height: 24),
              ElevatedButton(
                onPressed: _canSubmit && !_isSubmitting ? _continue : null,
                style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(54)),
                child: _isSubmitting
                    ? const SizedBox(
                        width: 22,
                        height: 22,
                        child: CircularProgressIndicator(strokeWidth: 2.4, valueColor: AlwaysStoppedAnimation(Colors.white)),
                      )
                    : const Text('Continue'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
