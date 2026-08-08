import 'package:flutter/material.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:geolocator/geolocator.dart';
import 'package:latlong2/latlong.dart' as latlong;
import '../../../../core/components/app_snackbar.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';

/// In-app route map — an OSM tile view centered on the officer's current
/// location (or a fixed destination pin when [destination] is supplied),
/// so navigation doesn't require leaving the app / backgrounding it.
class RouteMapPage extends StatefulWidget {
  const RouteMapPage({super.key, this.title = 'Route Map', this.destination, this.destinationLabel});

  final String title;
  final latlong.LatLng? destination;
  final String? destinationLabel;

  @override
  State<RouteMapPage> createState() => _RouteMapPageState();
}

class _RouteMapPageState extends State<RouteMapPage> {
  final _mapController = MapController();
  latlong.LatLng? _currentPosition;
  bool _isLoading = true;

  @override
  void initState() {
    super.initState();
    _loadLocation();
  }

  Future<void> _loadLocation() async {
    try {
      var permission = await Geolocator.checkPermission();
      if (permission == LocationPermission.denied) {
        permission = await Geolocator.requestPermission();
      }
      if (permission == LocationPermission.denied || permission == LocationPermission.deniedForever) {
        if (mounted) AppSnackbar.danger(context, 'Location permission is required to show the map');
        setState(() => _isLoading = false);
        return;
      }
      final position = await Geolocator.getCurrentPosition(locationSettings: const LocationSettings(accuracy: LocationAccuracy.medium));
      if (!mounted) return;
      setState(() {
        _currentPosition = latlong.LatLng(position.latitude, position.longitude);
        _isLoading = false;
      });
    } catch (_) {
      if (mounted) {
        AppSnackbar.danger(context, 'Could not fetch your current location');
        setState(() => _isLoading = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final center = widget.destination ?? _currentPosition ?? const latlong.LatLng(18.5204, 73.8567);

    return Scaffold(
      appBar: AppBar(title: Text(widget.title, maxLines: 1, overflow: TextOverflow.ellipsis)),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator())
          : Stack(
              children: [
                FlutterMap(
                  mapController: _mapController,
                  options: MapOptions(initialCenter: center, initialZoom: 14),
                  children: [
                    TileLayer(
                      urlTemplate: 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
                      userAgentPackageName: 'com.onefin.onefin_employee',
                    ),
                    MarkerLayer(
                      markers: [
                        if (widget.destination != null)
                          Marker(
                            point: widget.destination!,
                            width: 40,
                            height: 40,
                            child: const Icon(Icons.location_on, color: AppColors.danger, size: 40),
                          ),
                        if (_currentPosition != null)
                          Marker(
                            point: _currentPosition!,
                            width: 34,
                            height: 34,
                            child: Container(
                              decoration: BoxDecoration(
                                color: AppColors.info,
                                shape: BoxShape.circle,
                                border: Border.all(color: Colors.white, width: 3),
                              ),
                              child: const Icon(Icons.navigation, color: Colors.white, size: 16),
                            ),
                          ),
                      ],
                    ),
                  ],
                ),
                if (widget.destinationLabel != null)
                  Positioned(
                    left: AppSpacing.md,
                    right: AppSpacing.md,
                    bottom: AppSpacing.md,
                    child: Container(
                      padding: const EdgeInsets.all(AppSpacing.md),
                      decoration: BoxDecoration(
                        color: Colors.white,
                        borderRadius: BorderRadius.circular(AppSpacing.radiusMd),
                        border: Border.all(color: AppColors.border),
                      ),
                      child: Row(
                        children: [
                          const Icon(Icons.location_on, color: AppColors.danger),
                          const SizedBox(width: AppSpacing.sm),
                          Expanded(
                            child: Text(
                              widget.destinationLabel!,
                              style: Theme.of(context).textTheme.bodyMedium,
                              maxLines: 2,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
              ],
            ),
    );
  }
}
