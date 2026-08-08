/// Global on/off switch for real backend calls. When true, auth and KYC
/// submission run entirely as local mock simulations (as the app
/// originally worked) instead of hitting the Go backend — useful when the
/// backend/Postgres/`adb reverse` setup isn't available. The real
/// networking code (ApiClient, AuthApiService, KycApiService) is left
/// fully in place; flip this back to false to re-enable it.
class AppConfig {
  AppConfig._();

  static const bool useMockBackend = false;
}
