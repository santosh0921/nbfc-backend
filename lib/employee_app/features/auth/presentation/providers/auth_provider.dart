import 'package:flutter/material.dart';
import 'package:local_auth/local_auth.dart';
import '../../../../core/constants/employee_app_module.dart';
import '../../../../core/error/failure.dart';
import '../../../../core/storage/local_storage_service.dart';
import '../../../../core/usecase/usecase.dart';
import '../../domain/entities/employee_entity.dart';
import '../../domain/repositories/auth_repository.dart';
import '../../domain/usecases/login_usecase.dart';

enum AuthStatus { initial, authenticating, authenticated, unauthenticated, error }

class AuthProvider extends ChangeNotifier {
  AuthProvider({
    required LoginUseCase loginUseCase,
    required BiometricLoginUseCase biometricLoginUseCase,
    required LogoutUseCase logoutUseCase,
    required ForgotPasswordUseCase forgotPasswordUseCase,
    required AuthRepository authRepository,
    required LocalStorageService localStorage,
    this.module = EmployeeAppModule.verification,
  })  : _loginUseCase = loginUseCase,
        _biometricLoginUseCase = biometricLoginUseCase,
        _logoutUseCase = logoutUseCase,
        _forgotPasswordUseCase = forgotPasswordUseCase,
        _authRepository = authRepository,
        _localStorage = localStorage {
    _restoreSession();
  }

  final LoginUseCase _loginUseCase;
  final BiometricLoginUseCase _biometricLoginUseCase;
  final LogoutUseCase _logoutUseCase;
  final ForgotPasswordUseCase _forgotPasswordUseCase;
  final AuthRepository _authRepository;
  final LocalStorageService _localStorage;
  final _localAuth = LocalAuthentication();

  /// Which employee module (Verification or Recovery) this login screen
  /// was reached through — set once at construction time from the module
  /// the user picked on the role-selection screen. Used to gate login by
  /// the employee's actual [EmployeeRole] so e.g. a Collection Agent can't
  /// sign in through the Verification module and vice versa.
  final EmployeeAppModule module;

  /// The modules an employee with [role] is permitted to sign into.
  /// Field verification officers and supervisors work the Verification
  /// module (supervisors oversee verification officers); collection
  /// agents work the Recovery module.
  static bool _roleAllowedInModule(EmployeeRole role, EmployeeAppModule module) {
    switch (role) {
      case EmployeeRole.fieldVerificationOfficer:
      case EmployeeRole.supervisor:
        return module == EmployeeAppModule.verification;
      case EmployeeRole.collectionAgent:
        return module == EmployeeAppModule.recovery;
    }
  }

  static String _moduleName(EmployeeAppModule module) =>
      module == EmployeeAppModule.verification ? 'Verification' : 'Recovery';

  static String _mismatchMessage(EmployeeRole role, EmployeeAppModule attemptedModule) {
    final registeredModule = role == EmployeeRole.collectionAgent ? EmployeeAppModule.recovery : EmployeeAppModule.verification;
    return 'This account is registered for ${_moduleName(registeredModule)}, not ${_moduleName(attemptedModule)}. '
        'Please go back and select the correct role.';
  }

  static const _kRememberedIdKey = 'remembered_employee_id';
  static const _kProfileCompletedPrefix = 'profile_completed_';

  AuthStatus status = AuthStatus.initial;
  EmployeeEntity? employee;
  String? errorMessage;
  bool rememberMe = false;
  String? rememberedIdentifier;

  /// True once an employee is authenticated but hasn't completed their
  /// first-time profile setup yet — the router redirects here before
  /// letting them reach the dashboard.
  bool needsProfileSetup = false;

  Future<void> _restoreSession() async {
    rememberedIdentifier = await _localStorage.getString(_kRememberedIdKey);
    rememberMe = rememberedIdentifier != null;
    final session = await _authRepository.currentSession();
    if (session != null) {
      if (!_roleAllowedInModule(session.role, module)) {
        // A session from a different module's login is on this device
        // (e.g. switched from Recovery to Verification role-select) —
        // don't silently let it through the wrong shell.
        await _authRepository.logout();
        status = AuthStatus.unauthenticated;
        notifyListeners();
        return;
      }
      employee = session;
      status = AuthStatus.authenticated;
      await _refreshProfileSetupFlag();
    } else {
      status = AuthStatus.unauthenticated;
    }
    notifyListeners();
  }

  Future<void> _refreshProfileSetupFlag() async {
    if (employee == null) return;
    final completed = await _localStorage.getBool('$_kProfileCompletedPrefix${employee!.id}');
    needsProfileSetup = completed != true;
  }

  Future<void> completeProfileSetup() async {
    if (employee == null) return;
    await _localStorage.setBool('$_kProfileCompletedPrefix${employee!.id}', true);
    needsProfileSetup = false;
    notifyListeners();
  }

  Future<bool> get canUseBiometrics async {
    try {
      final supported = await _localAuth.isDeviceSupported();
      final canCheck = await _localAuth.canCheckBiometrics;
      return supported && canCheck;
    } catch (_) {
      return false;
    }
  }

  Future<void> login({required String identifier, required String password, bool remember = false}) async {
    status = AuthStatus.authenticating;
    errorMessage = null;
    notifyListeners();

    final result = await _loginUseCase(LoginParams(identifier: identifier, password: password));
    await result.when(
      success: (data) async {
        if (!_roleAllowedInModule(data.role, module)) {
          await _authRepository.logout();
          employee = null;
          status = AuthStatus.error;
          errorMessage = _mismatchMessage(data.role, module);
          return;
        }
        employee = data;
        status = AuthStatus.authenticated;
        rememberMe = remember;
        if (remember) {
          await _localStorage.setString(_kRememberedIdKey, identifier);
        } else {
          await _localStorage.remove(_kRememberedIdKey);
        }
      },
      failure: (f) async {
        status = AuthStatus.error;
        errorMessage = f.message;
      },
    );
    if (status == AuthStatus.authenticated) await _refreshProfileSetupFlag();
    notifyListeners();
  }

  Future<void> loginWithBiometrics() async {
    status = AuthStatus.authenticating;
    errorMessage = null;
    notifyListeners();

    try {
      final authenticated = await _localAuth.authenticate(
        localizedReason: 'Authenticate to access your ONEFIN employee account',
        options: const AuthenticationOptions(biometricOnly: true, stickyAuth: true),
      );
      if (!authenticated) {
        status = AuthStatus.unauthenticated;
        notifyListeners();
        return;
      }
    } catch (_) {
      status = AuthStatus.error;
      errorMessage = 'Biometric authentication is not available on this device.';
      notifyListeners();
      return;
    }

    final result = await _biometricLoginUseCase(const NoParams());
    await result.when(
      success: (data) async {
        if (!_roleAllowedInModule(data.role, module)) {
          await _authRepository.logout();
          employee = null;
          status = AuthStatus.error;
          errorMessage = _mismatchMessage(data.role, module);
          return;
        }
        employee = data;
        status = AuthStatus.authenticated;
      },
      failure: (f) async {
        status = AuthStatus.error;
        errorMessage = f.message;
      },
    );
    if (status == AuthStatus.authenticated) await _refreshProfileSetupFlag();
    notifyListeners();
  }

  Future<Failure?> forgotPassword(String identifier) async {
    final result = await _forgotPasswordUseCase(ForgotPasswordParams(identifier: identifier));
    return result.when(success: (_) => null, failure: (f) => f);
  }

  Future<void> logout() async {
    await _logoutUseCase(const NoParams());
    employee = null;
    status = AuthStatus.unauthenticated;
    needsProfileSetup = false;
    notifyListeners();
  }
}
