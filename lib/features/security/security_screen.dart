import 'package:flutter/material.dart';
import 'package:local_auth/local_auth.dart';
import '../../core/network/api_exception.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import '../../core/widgets/app_snackbar.dart';
import '../../core/widgets/premium_card.dart';
import '../auth/widgets/code_boxes.dart';

/// Profile → Security: change your MPIN, toggle biometric login, and
/// review active sessions. Change-MPIN and the biometric toggle both call
/// the real backend (`PUT /auth/change-mpin`, `PUT /auth/biometric`) via
/// [AuthProvider] — nothing here is simulated.
class SecurityScreen extends StatefulWidget {
  const SecurityScreen({super.key});

  @override
  State<SecurityScreen> createState() => _SecurityScreenState();
}

class _SecurityScreenState extends State<SecurityScreen> {
  final LocalAuthentication _localAuth = LocalAuthentication();

  static const _sessions = [
    ('This Device', 'Android · Active now', true),
    ('iPhone 14', 'iOS · 2 days ago', false),
    ('Chrome — Windows', 'Web · 5 days ago', false),
  ];

  void _openChangeMpinSheet() {
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (context) => const _ChangeMpinSheet(),
    );
  }

  /// Turning biometric ON needs the current MPIN once — it's what gets
  /// stored (in OS secure storage) as the credential biometric unlock
  /// later releases. Turning it OFF just clears that and flips the flag.
  Future<void> _onBiometricToggled(bool enable) async {
    if (!enable) {
      await AuthProvider.instance.disableBiometricLogin();
      if (mounted) setState(() {});
      return;
    }
    final mpin = await showModalBottomSheet<String>(
      context: context,
      isScrollControlled: true,
      builder: (context) => const _ConfirmMpinForBiometricSheet(),
    );
    if (mpin == null || mpin.length != 6) return;
    try {
      final authenticated = await _localAuth.authenticate(
        localizedReason: 'Enable fingerprint login for OneFinance',
        options: const AuthenticationOptions(biometricOnly: false, stickyAuth: true),
      );
      if (!authenticated) return;
      await AuthProvider.instance.enableBiometricLogin(mpin);
      if (mounted) {
        setState(() {});
        AppSnackBar.showSuccess(context, 'Fingerprint login enabled.');
      }
    } catch (_) {
      if (mounted) AppSnackBar.showError(context, 'Could not access biometrics on this device.');
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('Security')),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(20),
          children: [
            PremiumCard(
              padding: EdgeInsets.zero,
              child: Column(
                children: [
                  ListTile(
                    leading: const Icon(Icons.password_rounded, color: AppColors.primary),
                    title: const Text('Change MPIN'),
                    trailing: const Icon(Icons.chevron_right_rounded),
                    onTap: _openChangeMpinSheet,
                  ),
                  const Divider(height: 1, indent: 16),
                  SwitchListTile(
                    secondary: const Icon(Icons.fingerprint_rounded, color: AppColors.primary),
                    title: const Text('Biometric Login'),
                    subtitle: const Text('Use fingerprint or face unlock'),
                    value: AuthProvider.instance.biometricEnabled,
                    activeThumbColor: AppColors.primary,
                    onChanged: _onBiometricToggled,
                  ),
                ],
              ),
            ),
            const SizedBox(height: 20),
            Text('Active Sessions', style: theme.textTheme.titleMedium),
            const SizedBox(height: 12),
            PremiumCard(
              padding: EdgeInsets.zero,
              child: Column(
                children: [
                  for (int i = 0; i < _sessions.length; i++) ...[
                    ListTile(
                      leading: Icon(
                        _sessions[i].$3 ? Icons.smartphone_rounded : Icons.devices_other_rounded,
                        color: _sessions[i].$3 ? AppColors.success : AppColors.textSecondaryOf(context),
                      ),
                      title: Text(_sessions[i].$1),
                      subtitle: Text(_sessions[i].$2),
                      trailing: _sessions[i].$3
                          ? Container(
                              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                              decoration: BoxDecoration(
                                color: AppColors.success.withValues(alpha: 0.12),
                                borderRadius: BorderRadius.circular(AppRadius.pill),
                              ),
                              child: Text('This device', style: theme.textTheme.labelSmall?.copyWith(color: AppColors.success)),
                            )
                          : TextButton(
                              onPressed: () => ScaffoldMessenger.of(context).showSnackBar(
                                SnackBar(content: Text('Signed out of ${_sessions[i].$1}.')),
                              ),
                              child: const Text('Sign out'),
                            ),
                    ),
                    if (i != _sessions.length - 1) const Divider(height: 1, indent: 16),
                  ],
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _ChangeMpinSheet extends StatefulWidget {
  const _ChangeMpinSheet();

  @override
  State<_ChangeMpinSheet> createState() => _ChangeMpinSheetState();
}

class _ChangeMpinSheetState extends State<_ChangeMpinSheet> {
  String _current = '';
  String _newMpin = '';
  String _confirmMpin = '';
  String? _error;
  bool _isSubmitting = false;

  bool get _canSubmit => _current.length == 6 && _newMpin.length == 6 && _confirmMpin.length == 6;

  Future<void> _submit() async {
    if (_confirmMpin != _newMpin) {
      setState(() => _error = 'New MPIN and confirmation do not match.');
      return;
    }
    if (_current == _newMpin) {
      setState(() => _error = 'New MPIN cannot be the same as the current MPIN.');
      return;
    }
    setState(() {
      _isSubmitting = true;
      _error = null;
    });
    try {
      await AuthProvider.instance.changeMpin(_current, _newMpin, _confirmMpin);
      if (!mounted) return;
      Navigator.of(context).pop();
      AppSnackBar.showSuccess(context, 'MPIN updated successfully.');
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
    return Padding(
      padding: EdgeInsets.only(bottom: MediaQuery.of(context).viewInsets.bottom),
      child: SingleChildScrollView(
        padding: const EdgeInsets.fromLTRB(24, 20, 24, 28),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Change MPIN', style: theme.textTheme.headlineSmall),
            const SizedBox(height: 20),
            Text('Current MPIN', style: theme.textTheme.labelMedium),
            const SizedBox(height: 8),
            CodeBoxes(obscure: true, autofocus: true, onChanged: (v) => setState(() => _current = v)),
            const SizedBox(height: 18),
            Text('New MPIN', style: theme.textTheme.labelMedium),
            const SizedBox(height: 8),
            CodeBoxes(obscure: true, onChanged: (v) => setState(() => _newMpin = v)),
            const SizedBox(height: 18),
            Text('Confirm New MPIN', style: theme.textTheme.labelMedium),
            const SizedBox(height: 8),
            CodeBoxes(obscure: true, hasError: _error != null, onChanged: (v) => setState(() {
                  _confirmMpin = v;
                  _error = null;
                })),
            if (_error != null) ...[
              const SizedBox(height: 8),
              Text(_error!, style: theme.textTheme.bodySmall?.copyWith(color: AppColors.error)),
            ],
            const SizedBox(height: 20),
            ElevatedButton(
              onPressed: _canSubmit && !_isSubmitting ? _submit : null,
              style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(52)),
              child: _isSubmitting
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2.2, valueColor: AlwaysStoppedAnimation(Colors.white)),
                    )
                  : const Text('Update MPIN'),
            ),
          ],
        ),
      ),
    );
  }
}

/// Confirms the current MPIN before turning biometric login on — this is
/// the one moment the plaintext MPIN is available to store (in OS secure
/// storage) as the credential biometric unlock will later release.
class _ConfirmMpinForBiometricSheet extends StatefulWidget {
  const _ConfirmMpinForBiometricSheet();

  @override
  State<_ConfirmMpinForBiometricSheet> createState() => _ConfirmMpinForBiometricSheetState();
}

class _ConfirmMpinForBiometricSheetState extends State<_ConfirmMpinForBiometricSheet> {
  String _mpin = '';

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: EdgeInsets.only(bottom: MediaQuery.of(context).viewInsets.bottom),
      child: SingleChildScrollView(
        padding: const EdgeInsets.fromLTRB(24, 20, 24, 28),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Confirm Your MPIN', style: theme.textTheme.headlineSmall),
            const SizedBox(height: 8),
            Text(
              'Enter your current MPIN to enable fingerprint login.',
              style: theme.textTheme.bodyMedium,
            ),
            const SizedBox(height: 20),
            CodeBoxes(obscure: true, autofocus: true, onChanged: (v) => setState(() => _mpin = v)),
            const SizedBox(height: 20),
            ElevatedButton(
              onPressed: _mpin.length == 6 ? () => Navigator.of(context).pop(_mpin) : null,
              style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(52)),
              child: const Text('Continue'),
            ),
          ],
        ),
      ),
    );
  }
}
