# Структура проекта Vesper Launcher (Golang + Wails v2)

Vesper Launcher — современный, легковесный лаунчер Minecraft Java Edition, перенесённый с C# (.NET 10 / Photino) на Golang (Wails v2 + WebKitGTK-4.1) с сохранением 100% визуального стиля (React 18 + Vite, WebGL Liquid Glass lenses, динамические фоны).

---

## 1. Дерево файлов и каталогов

```
Vesper-Launcher-Go/
├── main.go                     # Точка входа приложения, инициализация Wails runtime, восстановление размера окна
├── app.go                      # Привязка контекста Wails, RPC-методы SendCommand/SendMessage, фоновый heartbeat
├── go.mod                      # Модуль Go и внешние зависимости
├── go.sum                      # Контрольные суммы зависимостей Go
├── wails.json                  # Конфигурация Wails v2 (метаданные, пути сборки, ассеты)
├── build.sh                    # Скрипт быстрой сборки фронтенда и нативного бинарника
├── STRUCTURE.md                # Данный документ архитектуры и структуры проекта
├── build/                      # Каталог сборки Wails
│   ├── bin/                    # Скомпилированный бинарный файл (VesperLauncher)
│   └── assets/                 # Иконки приложения, Info.plist и метаданные окон
├── bundled/                    # Встроенные предустановленные версии и библиотеки
│   ├── BundledVersions/        # Встроенная сборка Forge + OptiFine 1.16.5
│   └── BundledLibraries/       # Офлайн-библиотеки для запуска встроенной версии
├── internal/                   # Внутренние модули ядра лаунчера (Go)
│   ├── account/                # Клиент авторизации Vesper Account API
│   │   └── service.go          # PBKDF2-SHA256 хеширование, регистрация, вход, синхронизация друзей
│   ├── auth/                   # Подсистемы аутентификации игроков
│   │   ├── microsoft.go        # Полный OAuth 2.0 Device Code Flow + Xbox Live + Minecraft Services
│   │   └── offline.go          # Офлайн-авторизация и генерация UUID v3
│   ├── bridge/                 # Диспетчер команд UI -> Ядро лаунчера
│   │   └── bridge.go           # Обработка всех событий фронтенда, RPC и рассылка снепшотов
│   ├── java/                   # Детектор сред выполнения Java
│   │   └── detector.go         # Сканирование системы на Java 8, 17, 21 и проверка совместимости
│   ├── minecraft/              # Полноценный движок установки и запуска Minecraft
│   │   ├── manifest.go         # Парсинг манифестов версий Mojang, правил (rules) и ОС-фильтров
│   │   ├── downloader.go       # Параллельная многопоточная загрузка ассетов/библиотек с SHA-1 верификацией
│   │   ├── natives.go          # Распаковка динамических нативных библиотек (.so, .dll, .dylib)
│   │   ├── args.go             # Сборка Classpath, JVM аргументов и игровых флагов
│   │   ├── launcher.go         # Порождение процесса игры, стриминг логов в UI, авто-сворачивание
│   │   └── installer.go        # Координатор конвейера: манифест -> библиотеки -> ассеты -> клиент
│   ├── models/                 # Строгие контракты данных и DTO
│   │   └── types.go            # Модели снепшотов состояния лаунчера для интерфейса React
│   ├── mods/                   # Интеграция с каталогами модов
│   │   └── modrinth.go         # Modrinth v2 API (поиск модов, ресурспаков, шейдеров и сборок)
│   ├── platform/               # Кроссплатформенные пути файловой системы
│   │   └── paths.go            # Разрешение путей .minecraft, AppData/Roaming, XDG_CONFIG_HOME
│   ├── state/                  # Менеджер реактивного состояния лаунчера
│   │   └── state.go            # Потокобезопасный (sync.RWMutex) стор состояния, JSON-персистентность
│   └── updater/                # Модуль проверки и установки обновлений
│       └── updater.go          # Проверка новых релизов через GitHub API
└── frontend/                   # Интерфейс пользователя (React 18 + Vite + TypeScript)
    ├── index.html              # HTML-каркас приложения
    ├── package.json            # Зависимости интерфейса (React, Lucide, Vite)
    ├── vite.config.ts          # Конфигурация Vite bundler
    ├── tsconfig.json           # Конфигурация TypeScript
    ├── public/                 # Статические ресурсы (фоны, звуки, иконки, скины)
    │   └── assets/             # bg-day, bg-night, bg-sunrise, bg-sunset, steve.png, wordmarks
    ├── dist/                   # Скомпилированный продакшен-бандл (встраивается в Go binary через embed.FS)
    └── src/                    # Исходный код React UI
        ├── main.tsx            # Точка входа React приложения
        ├── App.tsx             # Главный компонент лаунчера, оркестрация панелей и событий
        ├── bridge.ts           # Адаптер моста (Wails v2 runtime Events + fallback RPC)
        ├── types.ts            # TypeScript интерфейсы HostSnapshot, LauncherSnapshot, настроек
        ├── styles.css          # Стили лаунчера, стеклянные панели, адаптивность, анимации
        ├── components/         # Компоненты интерфейса
        │   ├── common/         # Переиспользуемые элементы (AvatarImage, PanelHeader, ToggleCard, SelectFields)
        │   └── panels/         # Панели лаунчера:
        │       ├── AccountPanel.tsx      # Управление аккаунтом (Вход / Регистрация / Офлайн / Смена аватара)
        │       ├── BackgroundPanel.tsx   # Каталог фонов (динамические по времени суток / свои фоны)
        │       ├── EmptyPanel.tsx        # Заглушка пустой панели
        │       ├── FriendsPanel.tsx      # Система друзей (список, онлайн-статусы, запросы, добавление)
        │       ├── ModsPanel.tsx         # Поиск и установка модов (Modrinth / CurseForge)
        │       ├── NewsPanel.tsx         # Лента новостей лаунчера
        │       ├── NotificationsPanel.tsx # Уведомления системы
        │       ├── SettingsPanel.tsx     # Настройки (Память, Java, лаунчер, оформление, стекло)
        │       ├── SkinPanel.tsx         # Выбор скина, смена модели Steve/Alex, импорт PNG
        │       └── UpdatesPanel.tsx      # Экран обновлений лаунчера
        └── vendor/             # Сторонние оптимизированные графические библиотеки
            ├── liquidGL.js     # Движок WebGL Liquid Glass с dirty-checked рендером и быстрым кэшированием геометрии
            └── html2canvas.min.js # Запасной захватчик DOM-элементов
```

---

## 2. Назначение и обязанности ключевых пакетов

### 2.1. Точка входа (`main.go` & `app.go`)
* `main.go`:
  * Инициализирует C-конструктор для среды Linux (`GDK_BACKEND=x11`, `WEBKIT_DISABLE_DMABUF_RENDERER=1`, предотвращающие вылеты драйверов NVIDIA на Wayland/X11).
  * Считывает сохранённую геометрию окна (`WindowWidth`, `WindowHeight`) из `LauncherState`.
  * Запускает приложение через `wails.Run()` с нативной рамкой окна на Linux/macOS и прозрачным фоном.
* `app.go`:
  * Привязывает `App` к контексту Wails.
  * Экспортирует методы `SendCommand(command, payload)` и `SendMessage(rawJson)` в JavaScript рантайм `window.go.main.App`.
  * Запускает фоновый heartbeat-тикер рассылки состояния (10 с) в качестве резерва для реактивных событий.

### 2.2. Менеджер состояния (`internal/state/state.go`)
* Центральное хранилище данных приложения, защищённое `sync.RWMutex`.
* Хранит:
  * Текущий статус запуска (`StatusText`, `ProgressPercent`, `IsBusy`, `IsGameRunning`).
  * Выбранную версию Minecraft (`SelectedVersion`) и каталог доступных версий (`Versions`).
  * Настройки памяти (`JavaMemoryMb`), путь к Java (`JavaPath`), аргументы JVM (`JvmArgs`).
  * Параметры интерфейса: активная вкладка, положение формы входа (`LoginFormPlacementId`), режим Java (`JavaRuntimeMode`), режим каталога (`LauncherDirectoryViewId`).
  * Размеры окна (`WindowWidth`, `WindowHeight`) с автоматическим сохранением.
* Автоматически сохраняет и восстанавливает настройки из файла:
  `~/.config/VesperLauncher/launcher-state.json`.

### 2.3. Мост взаимодействия (`internal/bridge/bridge.go`)
* Слушает команды от React UI по протоколу Wails Events (`vesper:command`) и через прямой RPC (`App.SendCommand`).
* Маршрутизирует более 30 типов команд:
  * Управление окнами: `host.windowsize`, `host.close`, `host.minimize`.
  * Секции лаунчера: `shell.opensection`, `shell.closesection`.
  * Запуск игры: `main.launch`, `main.selectversionkey`, `main.openprofilefolder`.
  * Аккаунт: `account.submit`, `account.logout`, `account.microsoft`, `account.pickavatar`.
  * Настройки: `settings.selecttab`, `settings.setmemory`, `settings.settoggle`, `settings.settext`, `settings.setoption`, `settings.opengamedirectory`.
  * Скины: `skin.importdialog`, `skin.setmodel`, `skin.clear`, `skin.openfolder`.
  * Фоны: `background.setpreset`, `background.reset`, `background.openfolder`.
  * Моды: `mods.setsearch`, `mods.selectcategory`, `mods.setprovider`, `mods.toggleitem`.
* Мгновенно рассылает обновлённый `HostSnapshot` всем подписчикам интерфейса через `EventsEmit("snapshot", snap)` и `EventsEmit("vesper:message", envelope)`.

### 2.4. Движок запуска Minecraft (`internal/minecraft/`)
* **`manifest.go`**: Загружает и кэширует официальный `version_manifest_v2.json` от Mojang, скачивает JSON конкретной версии, парсит правила (`rules`) библиотек с проверкой условий текущей операционной системы (Linux, Windows, macOS).
* **`downloader.go`**: Реализует многопоточную параллельную загрузку файлов клиентов, библиотек и ассетов с контролем контрольных сумм по SHA-1 и поддержкой прогресс-бара.
* **`natives.go`**: Извлекает платформозависимые библиотеки `.so` / `.dll` / `.dylib` из jar-архивов во временную папку `natives/`.
* **`args.go`**: Конструирует финальный список параметров командной строки JVM, формирует Classpath (с разделителем `:` для Linux/macOS и `;` для Windows), подставляет аргументы Mojang (`${auth_player_name}`, `${version_name}`, `${game_directory}`, `${assets_root}` и др.).
* **`launcher.go`**: Порождает процесс `exec.Command`, перенаправляет стандартные потоки вывода в каналы логирования, управляет автоматическим скрытием и восстановлением окна лаунчера после выхода из игры.

### 2.5. Графика и визуальный движок (`frontend/src/vendor/liquidGL.js`)
* Реализует аппаратное преломление света (Liquid Glass / Refraction) на WebGL 2.0.
* **Оптимизации производительности**:
  * **Zero Layout Thrashing**: метод `render()` использует кэшированные координаты и не вызывает `getBoundingClientRect()` и `getComputedStyle()` во время циклов анимации.
  * **Dirty-checking рендер**: когда интерфейс статичен, WebGL-шейдер не производит холостых перерисовок и засыпает, снижая нагрузку на CPU до <1%.
  * **Очистка памяти**: отмонтированные кнопки автоматически удаляются из пула линз (`lens.destroy()`).
  * **Fast-path фон**: снимок фона рендерится напрямую в 2D Canvas через `Image.drawImage` за <0.2 мс вместо тяжёлого растрирования DOM `html2canvas`.

---

## 3. Сборка и запуск проекта

### Требования к окружению:
* Go 1.22+
* Node.js 18+ и npm
* Wails v2 CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
* Linux: `webkit2gtk4.1-devel` (Fedora: `sudo dnf install webkit2gtk4.1-devel`, Ubuntu: `sudo apt install libwebkit2gtk-4.1-dev`)

### Команды:
```bash
# 1. Сборка и компиляция (одной командой через скрипт)
./build.sh

# Либо вручную:
cd frontend && npm run build && cd ..
wails build -tags "webkit2_41"

# 2. Запуск собранного бинарника
./build/bin/VesperLauncher
```
