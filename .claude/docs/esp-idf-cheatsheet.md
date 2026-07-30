# ESP-IDF cheatsheet (project-flavored)

## Logging

```cpp
#include "esp_log.h"
static const char* TAG = "MyComponent";

ESP_LOGE(TAG, "fatal: %s", reason);
ESP_LOGW(TAG, "degraded: %d", code);
ESP_LOGI(TAG, "lifecycle event");
ESP_LOGD(TAG, "value=%d", x);

ESP_LOG_BUFFER_HEX(TAG, buf, len);
```

Set per-tag level at runtime: `esp_log_level_set("MyComponent", ESP_LOG_DEBUG);`

## NVS (non-volatile storage)

```cpp
#include "nvs_flash.h"
#include "nvs.h"

ESP_ERROR_CHECK(nvs_flash_init());
nvs_handle_t h;
ESP_ERROR_CHECK(nvs_open("storage", NVS_READWRITE, &h));
nvs_set_str(h, "key", "value");
nvs_commit(h);
nvs_close(h);
```

This project uses SD for books and bookmarks. NVS is the planned home for Wi-Fi credentials and any companion-app pairing secret.

## HTTP client

```cpp
esp_http_client_config_t cfg = {
    .url = "https://example.com",
    .cert_pem = root_ca_pem,        // string must outlive the request
    .timeout_ms = 10000,
};
auto client = esp_http_client_init(&cfg);
esp_http_client_set_method(client, HTTP_METHOD_GET);
esp_err_t err = esp_http_client_perform(client);
int status = esp_http_client_get_status_code(client);
esp_http_client_cleanup(client);
```

ESRead does not yet have an HTTP client wrapper component. When the Wi-Fi book intake path lands, add one — don't sprinkle `esp_http_client_*` calls across components.

## FreeRTOS task

```cpp
void my_task(void* arg)
{
    while (true)
    {
        // work
        vTaskDelay(pdMS_TO_TICKS(1000));
    }
}

xTaskCreatePinnedToCore(my_task, "my", 8192, nullptr, 5, nullptr, APP_CPU_NUM);
//                            name  stack pri        handle  core
```

**Stack size guidance:** simple polling → 4 KB; JSON parsing → 6 KB; TLS / mbedTLS → 8 KB+; LVGL/display → 12 KB+.

## Kconfig consumption

```cpp
#include "sdkconfig.h"
const char* wifi_ssid = CONFIG_WIFI_SSID;
```

Strings from Kconfig are compile-time `const char*`. To compare safely: `if (strcmp(wifi_ssid, "No Key") == 0) { /* unset */ }`.

## Common headers cheatsheet

```cpp
#include "esp_log.h"          // logging
#include "esp_err.h"          // esp_err_t, ESP_ERROR_CHECK
#include "esp_system.h"       // esp_restart, MAC
#include "esp_event.h"        // default loop
#include "esp_wifi.h"         // wifi_init_config_t etc.
#include "esp_netif.h"        // networking
#include "esp_http_client.h"  // HTTP
#include "esp_https_ota.h"    // OTA
#include "nvs_flash.h"        // NVS init
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "freertos/queue.h"
#include "freertos/event_groups.h"
#include "driver/gpio.h"
#include "driver/i2c_master.h"  // new I2C driver (NOT i2c.h)
```

## std::expected idioms

```cpp
std::expected<int, MyError> compute();

auto r = compute();
if (!r) return std::unexpected(r.error());
int value = *r;          // or r.value()

// chain (C++23 monadic ops):
auto final = compute()
    .and_then([](int x) -> std::expected<double, MyError> { return x * 1.5; })
    .or_else([](MyError e) -> std::expected<double, MyError> { return 0.0; });
```
