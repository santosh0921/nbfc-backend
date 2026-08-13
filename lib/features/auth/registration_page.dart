import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';
import '../../core/providers/auth_provider.dart';
import '../apply/widgets/apply_form_field.dart';
import 'widgets/auth_illustration.dart';

/// Screen 2.5 of the login flow, right after OTP verification: collects
/// the new user's name, email, and date of birth so there's a real
/// profile behind the account from the start — saved to the backend
/// (`POST /auth/profile`) right after the MPIN step logs them in.
class RegistrationPage extends StatefulWidget {
  const RegistrationPage({super.key});

  @override
  State<RegistrationPage> createState() => _RegistrationPageState();
}

class _RegistrationPageState extends State<RegistrationPage> {
  final _formKey = GlobalKey<FormState>();
  final _nameController = TextEditingController();
  final _emailController = TextEditingController();
  final _dobController = TextEditingController();
  DateTime? _dobValue;

  @override
  void dispose() {
    _nameController.dispose();
    _emailController.dispose();
    _dobController.dispose();
    super.dispose();
  }

  Future<void> _pickDob() async {
    final picked = await showDatePicker(
      context: context,
      initialDate: DateTime(1995, 1, 1),
      firstDate: DateTime(1940),
      lastDate: DateTime.now().subtract(const Duration(days: 365 * 18)),
    );
    if (picked != null) {
      setState(() {
        _dobValue = picked;
        _dobController.text = DateFormat('d MMM yyyy').format(picked);
      });
    }
  }

  /// Splits the single "Full Name" field into first/middle/last, matching
  /// the backend's three-part name shape (`first_name`/`middle_name`/
  /// `last_name`) used for the name-collision check in `POST
  /// /auth/profile`. First word = first name, last word = last name,
  /// anything in between = middle name. Requires at least two words —
  /// enforced by [_validateName] before this ever runs.
  ({String first, String? middle, String last}) _splitName(String fullName) {
    final parts = fullName.trim().split(RegExp(r'\s+'));
    if (parts.length == 1) {
      return (first: parts.first, middle: null, last: parts.first);
    }
    final middle = parts.length > 2 ? parts.sublist(1, parts.length - 1).join(' ') : null;
    return (first: parts.first, middle: middle, last: parts.last);
  }

  String? _validateName(String? v) {
    final trimmed = v?.trim() ?? '';
    if (trimmed.isEmpty) return 'Enter your full name';
    if (!trimmed.contains(RegExp(r'\s'))) return 'Please enter your full name (first and last name)';
    return null;
  }

  void _continue() {
    if (!(_formKey.currentState?.validate() ?? false)) return;
    final name = _splitName(_nameController.text.trim());
    AuthProvider.instance
      ..pendingFirstName = name.first
      ..pendingMiddleName = name.middle
      ..pendingLastName = name.last
      ..pendingEmail = _emailController.text.trim()
      ..pendingDob = _dobValue != null ? DateFormat('yyyy-MM-dd').format(_dobValue!) : null;
    context.go('/create-mpin');
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(24, 8, 24, 24),
          child: Form(
            key: _formKey,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                const AuthIllustration(scene: AuthScene.mpin),
                const SizedBox(height: 24),
                Text('Create Your Profile', textAlign: TextAlign.center, style: theme.textTheme.headlineMedium),
                const SizedBox(height: 8),
                Text(
                  'Tell us a bit about yourself to finish setting up your account.',
                  textAlign: TextAlign.center,
                  style: theme.textTheme.bodyMedium,
                ),
                const SizedBox(height: 28),
                ApplyFormField(
                  label: 'Full Name',
                  controller: _nameController,
                  hint: 'Enter your full name',
                  validator: _validateName,
                ),
                ApplyFormField(
                  label: 'Email Address',
                  controller: _emailController,
                  keyboardType: TextInputType.emailAddress,
                  hint: 'you@example.com',
                  validator: (v) {
                    if (v == null || v.trim().isEmpty) return 'Enter your email';
                    if (!v.contains('@') || !v.contains('.')) return 'Enter a valid email';
                    return null;
                  },
                ),
                ApplyFormField(
                  label: 'Date of Birth',
                  controller: _dobController,
                  readOnly: true,
                  onTap: _pickDob,
                  hint: 'Select date',
                  validator: (v) => (v == null || v.isEmpty) ? 'Select your date of birth' : null,
                ),
                const SizedBox(height: 20),
                ElevatedButton(
                  onPressed: _continue,
                  style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(54)),
                  child: const Text('Continue'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
