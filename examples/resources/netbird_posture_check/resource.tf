resource "netbird_posture_check" "example" {
  name        = "TF Test"
  description = "Meow"

  netbird_version_check {
    min_version = "0.1.0"
  }

  os_version_check {
    android_min_version        = "0.0.0"
    ios_min_version            = "0.0.0"
    darwin_min_version         = "0.0.0"
    linux_min_kernel_version   = "0.0.0"
    windows_min_kernel_version = "0.0.1"
  }

  geo_location_check {
    locations = [
      {
        country_code = "EG"
      },
      {
        country_code = "DE"
      }
    ]
    action = "allow"
  }

  peer_network_range_check {
    ranges = [
      "0.0.0.0/0"
    ]

    action = "allow"
  }

  process_check {
    linux_path = "/some/path/in/linux"
    mac_path   = "/some/path/in/mac"
  }

  process_check {
    linux_path   = "/some/path/in/linux"
    windows_path = "C:\\some\\path\\in\\windows"
  }
}
