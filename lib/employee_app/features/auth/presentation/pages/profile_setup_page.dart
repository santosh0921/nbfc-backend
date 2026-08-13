import 'dart:io';

import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';
import 'package:provider/provider.dart';
import '../../../../core/components/app_snackbar.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/providers/agency_provider.dart';
import '../../../../core/router/route_names.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../core/widgets/app_text_field.dart';
import '../providers/auth_provider.dart';

/// Shown exactly once — right after an employee's very first successful
/// login — so their profile (contact details, emergency contact, photo) is
/// on file before they can reach the dashboard.
class ProfileSetupPage extends StatefulWidget {
  const ProfileSetupPage({super.key});

  @override
  State<ProfileSetupPage> createState() => _ProfileSetupPageState();
}

class _ProfileSetupPageState extends State<ProfileSetupPage> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _nameController;
  late final TextEditingController _phoneController;
  late final TextEditingController _emailController;
  late final TextEditingController _designationController;
  late final TextEditingController _branchController;
  final _emergencyContactController = TextEditingController();
  final _addressController = TextEditingController();
  String? _photoPath;
  bool _isSaving = false;

  @override
  void initState() {
    super.initState();
    final employee = context.read<AuthProvider>().employee;
    final agency = context.read<AgencyProvider>().selectedAgency;
    _nameController = TextEditingController(text: employee?.name ?? '');
    _phoneController = TextEditingController();
    _emailController = TextEditingController(text: employee?.email ?? '');
    _designationController = TextEditingController(text: employee?.designation ?? '');
    _branchController = TextEditingController(text: agency ?? employee?.branch ?? '');
  }

  @override
  void dispose() {
    _nameController.dispose();
    _phoneController.dispose();
    _emailController.dispose();
    _designationController.dispose();
    _branchController.dispose();
    _emergencyContactController.dispose();
    _addressController.dispose();
    super.dispose();
  }

  Future<void> _pickPhoto() async {
    try {
      final photo = await ImagePicker().pickImage(source: ImageSource.camera, imageQuality: 70, maxWidth: 600);
      if (photo != null) setState(() => _photoPath = photo.path);
    } catch (_) {
      if (mounted) AppSnackbar.danger(context, 'Could not open camera');
    }
  }

  Future<void> _save() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _isSaving = true);
    await context.read<AuthProvider>().completeProfileSetup();
    if (!mounted) return;
    setState(() => _isSaving = false);
    context.goNamed(RouteNames.dashboard);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.white,
      body: SafeArea(
        child: Form(
          key: _formKey,
          child: ListView(
            padding: const EdgeInsets.all(AppSpacing.lg),
            children: [
              Text('Complete Your Profile', style: Theme.of(context).textTheme.headlineSmall),
              const SizedBox(height: 4),
              Text(
                'Just this once — confirm your details so your reports and receipts carry the right information.',
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: AppColors.textSecondary),
              ),
              const SizedBox(height: AppSpacing.lg),
              Center(
                child: GestureDetector(
                  onTap: _pickPhoto,
                  child: Stack(
                    children: [
                      CircleAvatar(
                        radius: 44,
                        backgroundColor: AppColors.surfaceMuted,
                        backgroundImage: _photoPath != null ? FileImage(File(_photoPath!)) : null,
                        child: _photoPath == null ? const Icon(Icons.person_outline, size: 40, color: AppColors.textSecondary) : null,
                      ),
                      Positioned(
                        bottom: 0,
                        right: 0,
                        child: Container(
                          width: 28,
                          height: 28,
                          alignment: Alignment.center,
                          decoration: const BoxDecoration(color: AppColors.secondary, shape: BoxShape.circle),
                          child: const Icon(Icons.camera_alt, color: AppColors.primary, size: 14),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: AppSpacing.lg),
              AppTextField(
                label: 'Full Name',
                hint: 'Enter your full name',
                controller: _nameController,
                prefixIcon: Icons.badge_outlined,
                validator: (v) => (v == null || v.trim().isEmpty) ? 'Required' : null,
              ),
              const SizedBox(height: AppSpacing.md),
              AppTextField(
                label: 'Mobile Number',
                hint: 'Enter your mobile number',
                controller: _phoneController,
                prefixIcon: Icons.phone_outlined,
                keyboardType: TextInputType.phone,
                validator: (v) => (v == null || v.trim().length < 10) ? 'Enter a valid mobile number' : null,
              ),
              const SizedBox(height: AppSpacing.md),
              AppTextField(
                label: 'Email',
                hint: 'you@example.com',
                controller: _emailController,
                prefixIcon: Icons.email_outlined,
                keyboardType: TextInputType.emailAddress,
                validator: (v) => (v == null || !v.contains('@')) ? 'Enter a valid email' : null,
              ),
              const SizedBox(height: AppSpacing.md),
              AppTextField(label: 'Designation', hint: 'e.g. Field Verification Officer', controller: _designationController, prefixIcon: Icons.work_outline),
              const SizedBox(height: AppSpacing.md),
              AppTextField(label: 'Branch / Agency', hint: 'Enter your branch or agency', controller: _branchController, prefixIcon: Icons.apartment_outlined),
              const SizedBox(height: AppSpacing.md),
              AppTextField(
                label: 'Residential Address',
                hint: 'Enter your residential address',
                controller: _addressController,
                prefixIcon: Icons.home_outlined,
                validator: (v) => (v == null || v.trim().isEmpty) ? 'Required' : null,
              ),
              const SizedBox(height: AppSpacing.md),
              AppTextField(
                label: 'Emergency Contact',
                hint: 'Enter an emergency contact number',
                controller: _emergencyContactController,
                prefixIcon: Icons.emergency_outlined,
                keyboardType: TextInputType.phone,
                validator: (v) => (v == null || v.trim().length < 10) ? 'Enter a valid contact number' : null,
              ),
              const SizedBox(height: AppSpacing.xl),
              AppButton(label: 'Save & Continue', isLoading: _isSaving, onPressed: _save),
            ],
          ),
        ),
      ),
    );
  }
}
