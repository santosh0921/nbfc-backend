import 'dart:convert';
import '../../../../core/error/error_mapper.dart';
import '../../../../core/error/failure.dart';
import '../../../../core/error/result.dart';
import '../../../../core/storage/secure_storage_service.dart';
import '../../domain/entities/employee_entity.dart';
import '../../domain/repositories/auth_repository.dart';
import '../datasources/auth_remote_datasource.dart';
import '../models/employee_model.dart';

class AuthRepositoryImpl implements AuthRepository {
  AuthRepositoryImpl(this._remoteDataSource, this._secureStorage);

  final AuthRemoteDataSource _remoteDataSource;
  final SecureStorageService _secureStorage;

  static const _kSessionKey = 'employee_session';
  static const _kTokenKey = 'auth_token';

  @override
  Future<Result<EmployeeEntity>> login({required String name, required String password, required String module}) async {
    try {
      final result = await _remoteDataSource.login(name: name, password: password, module: module);
      await _persistSession(result.employee, result.token);
      return Success(result.employee);
    } catch (e) {
      return ResultFailure(ErrorMapper.map(e));
    }
  }

  @override
  Future<Result<void>> register({
    required String name,
    required String password,
    required String role,
    required String branchCity,
  }) async {
    try {
      await _remoteDataSource.register(name: name, password: password, role: role, branchCity: branchCity);
      return const Success(null);
    } catch (e) {
      return ResultFailure(ErrorMapper.map(e));
    }
  }

  /// There is no biometric-specific backend endpoint — biometric unlock
  /// simply re-admits the employee whose password-authenticated session
  /// (including its still-valid JWT, good for 12h) is already persisted on
  /// this device from a prior [login]. If no session was ever persisted,
  /// or it was cleared by [logout], there is nothing to resolve to.
  @override
  Future<Result<EmployeeEntity>> loginWithBiometrics() async {
    try {
      final session = await currentSession();
      final token = await _secureStorage.read(_kTokenKey);
      if (session == null || token == null) {
        throw const UnauthorizedFailure('No previous session found. Please log in with your employee ID.');
      }
      return Success(session);
    } catch (e) {
      return ResultFailure(ErrorMapper.map(e));
    }
  }

  @override
  Future<Result<void>> forgotPassword({required String identifier}) async {
    try {
      await _remoteDataSource.forgotPassword(identifier);
      return const Success(null);
    } catch (e) {
      return ResultFailure(ErrorMapper.map(e));
    }
  }

  @override
  Future<Result<void>> logout() async {
    await _secureStorage.delete(_kSessionKey);
    await _secureStorage.delete(_kTokenKey);
    return const Success(null);
  }

  @override
  Future<EmployeeEntity?> currentSession() async {
    final raw = await _secureStorage.read(_kSessionKey);
    if (raw == null) return null;
    return EmployeeModel.fromJson(jsonDecode(raw) as Map<String, dynamic>);
  }

  @override
  Future<bool> get hasSession async => await _secureStorage.read(_kSessionKey) != null;

  Future<void> _persistSession(EmployeeModel employee, String token) async {
    await _secureStorage.write(_kSessionKey, jsonEncode(employee.toJson()));
    await _secureStorage.write(_kTokenKey, token);
  }
}
