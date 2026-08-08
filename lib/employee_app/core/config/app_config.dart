enum AppEnv { development, staging, production }

class AppConfig {
  const AppConfig._({required this.env, required this.apiBaseUrl, required this.useMockData});

  final AppEnv env;
  final String apiBaseUrl;
  final bool useMockData;

  static late AppConfig current;

  static void init(AppConfig config) {
    current = config;
  }

  // Points at the Go backend in cmd/api. On a physical device this only
  // works behind `adb reverse tcp:8080 tcp:8080` (forwards the device's own
  // localhost:8080 to the dev machine's) — same setup the customer app's
  // ApiClient (lib/core/network/api_client.dart) documents and relies on.
  static const development = AppConfig._(
    env: AppEnv.development,
    apiBaseUrl: 'http://localhost:8080',
    useMockData: false,
  );

  static const production = AppConfig._(
    env: AppEnv.production,
    apiBaseUrl: 'https://api.onefin.com/v1',
    useMockData: false,
  );
}
