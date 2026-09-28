# Схема базы данных Go backend

Документ описывает live-схему PostgreSQL базы `app`, которой владеет Go
микросервис backend. Схема backend находится в `public`; её версия миграций в
live базе — `13`.

Таблицы `generator.*` и `generator.alembic_version` в этот документ не входят.
Они используют ту же PostgreSQL базу, но принадлежат сервису generator и
описаны отдельно в
[`../generator/dst.md`](../generator/dst.md).

## Карта связей

```text
users
  └── hypothesis
        ├── product
        │     ├── audience
        │     └── ad_campaign
        ├── file
        ├── hypothesis_launch
        │     ├── hypothesis_launch_resource
        │     └── temporal_outbox
        └── hypothesis_stop
              └── temporal_stop_outbox
```

`audience.generator_job_id` хранит идентификатор задачи generator как `TEXT`,
но foreign key между backend и generator не задан.

## `users`

Пользователь backend и владелец гипотез.

| Поле | Тип | Nullable | Default | Описание |
|---|---|---:|---|---|
| `id` | `TEXT` | нет | — | Primary key |
| `email` | `TEXT` | нет | — | Уникальный email |
| `password_hash` | `TEXT` | нет | — | Хэш пароля |
| `created_at` | `TIMESTAMP` | нет | `NOW()` | Время создания |

Ограничения и индексы:

- primary key: `id`;
- unique: `email`.

## `hypothesis`

Гипотеза, принадлежащая пользователю. На гипотезу ссылаются продукт, файлы и
операции запуска/остановки.

| Поле | Тип | Nullable | Default | Описание |
|---|---|---:|---|---|
| `id` | `TEXT` | нет | — | Primary key |
| `user_id` | `TEXT` | нет | — | Владелец, ссылка на `users.id` |
| `description` | `VARCHAR(1024)` | нет | — | Описание гипотезы |
| `status` | `TEXT` | нет | `'idle'` | `idle`, `launching`, `in_progress`, `stopping` или `requires_manual_intervention` |
| `created_at` | `TIMESTAMP` | нет | `NOW()` | Время создания |
| `active_launch_id` | `TEXT` | да | — | Текущий запуск, ссылка на `hypothesis_launch.id` |
| `active_stop_id` | `TEXT` | да | — | Текущая остановка, ссылка на `hypothesis_stop.id` |

Ограничения и индексы:

- `user_id` ссылается на `users.id` с `ON DELETE CASCADE`;
- `active_launch_id` ссылается на `hypothesis_launch.id` с `ON DELETE SET NULL`;
- `active_stop_id` ссылается на `hypothesis_stop.id` с `ON DELETE SET NULL`;
- индекс `hypothesis_user_id_idx` по `user_id`;
- `status` ограничен перечисленными значениями.

## `product`

Продукт и входные данные для генерации аудиторий. Для одной гипотезы может быть
только один продукт.

| Поле | Тип | Nullable | Default | Описание |
|---|---|---:|---|---|
| `id` | `TEXT` | нет | — | Primary key |
| `name` | `TEXT` | нет | — | Название продукта |
| `description` | `TEXT` | нет | — | Описание продукта |
| `category` | `TEXT` | нет | — | Категория продукта |
| `keywords` | `keyword[]` | нет | `'{}'` | Ключевые слова; элемент домена `keyword` не длиннее 64 символов |
| `price` | `JSONB` | нет | JSON со статусом `unknown` | Структурированная информация о стоимости |
| `link` | `VARCHAR(128)` | да | — | Ссылка на продукт |
| `hypothesis_id` | `TEXT` | нет | — | Ссылка на `hypothesis.id` |
| `brand` | `TEXT` | нет | — | Бренд |
| `experiment_goal` | `TEXT` | нет | — | Цель эксперимента |
| `target_action` | `TEXT` | нет | — | Целевое действие пользователя |
| `facts` | `JSONB` | нет | — | Факты о продукте |
| `eligibility_rules` | `JSONB` | нет | — | Правила допустимости аудитории |
| `allowed_channels` | `TEXT[]` | нет | — | Разрешённые каналы |
| `forbidden_claims` | `TEXT[]` | нет | — | Запрещённые утверждения |
| `known_constraints` | `TEXT[]` | нет | — | Известные ограничения |
| `country` | `VARCHAR(255)` | нет | — | Страна |
| `town` | `VARCHAR(255)` | да | — | Город или населённый пункт |
| `custom_category` | `VARCHAR(64)` | да | — | Пользовательская категория для `category = 'other'` |
| `goal_origin` | `TEXT` | нет | `'user_provided'` | `demo_assumption` или `user_provided` |
| `primary_metric` | `TEXT` | нет | `'target_action_rate'` | Основная метрика |
| `metric_direction` | `TEXT` | нет | `'increase'` | `increase` или `decrease` |
| `metric_numerator` | `TEXT` | нет | `'Unique users completing the target action'` | Числитель метрики |
| `metric_denominator` | `TEXT` | нет | `'Unique randomized users'` | Знаменатель метрики |
| `success_criterion` | `TEXT` | да | — | Критерий успеха |
| `key_fact_ids` | `TEXT[]` | нет | `'{}'` | Идентификаторы ключевых фактов |
| `facts_checked_at` | `TEXT` | да | — | Время или маркер проверки фактов |
| `unknowns` | `TEXT[]` | нет | `'{}'` | Неизвестные данные |
| `contactable_persona_ids` | `TEXT[]` | нет | `'{}'` | Идентификаторы доступных персон |
| `campaign_note` | `TEXT` | нет | `'Channels are not used for research or ranking.'` | Примечание для рекламной кампании |

Ограничения и индексы:

- `hypothesis_id` ссылается на `hypothesis.id` с `ON DELETE CASCADE`;
- `hypothesis_id` уникален;
- `category` принимает значения `electronics`, `clothing`, `food`, `furniture`,
  `beauty`, `sports`, `toys`, `books`, `automotive`, `health`, `home`, `office`,
  `garden`, `pets`, `music`, `banking_card`, `other`;
- если `category = 'other'`, `custom_category` обязательна, обрезается через
  `BTRIM` и имеет длину от 1 до 64 символов; для остальных категорий она должна
  быть `NULL`;
- `metric_direction` принимает `increase` или `decrease`;
- `goal_origin` принимает `demo_assumption` или `user_provided`.

## `audience`

Результат генерации и исследования одной персоны для продукта.

| Поле | Тип | Nullable | Default | Описание |
|---|---|---:|---|---|
| `id` | `TEXT` | нет | — | Primary key |
| `product_id` | `TEXT` | нет | — | Ссылка на `product.id` |
| `generator_job_id` | `TEXT` | нет | — | Идентификатор задачи generator; без foreign key |
| `persona_id` | `TEXT` | нет | — | Идентификатор персоны |
| `persona` | `JSONB` | нет | — | Данные персоны |
| `research` | `JSONB` | нет | — | Результат исследования |
| `metrics` | `JSONB` | нет | — | Метрики и оценки |
| `hypotheses` | `JSONB` | нет | — | Гипотезы для аудитории |
| `rank` | `INTEGER` | нет | — | Ранг от 1 до 3 |
| `score` | `DOUBLE PRECISION` | нет | — | Итоговая оценка |
| `created_at` | `TIMESTAMP` | нет | `NOW()` | Время создания |

Ограничения и индексы:

- `product_id` ссылается на `product.id` с `ON DELETE CASCADE`;
- `rank` находится между 1 и 3;
- комбинация `(generator_job_id, persona_id)` уникальна;
- индекс `audience_product_id_rank_idx` по `(product_id, rank)`.

## `file`

Метаданные файла, содержимое которого хранится в S3-совместимом хранилище.

| Поле | Тип | Nullable | Default | Описание |
|---|---|---:|---|---|
| `id` | `TEXT` | нет | — | Primary key |
| `hypothesis_id` | `TEXT` | нет | — | Ссылка на `hypothesis.id` |
| `type` | `TEXT` | нет | `'other'` | `product`, `brandbook`, `reference` или `other` |
| `filename` | `TEXT` | нет | — | Имя файла |
| `content_type` | `TEXT` | нет | — | MIME-тип |
| `size` | `BIGINT` | нет | — | Размер файла |
| `s3_key` | `TEXT` | нет | — | Уникальный ключ объекта в S3 |
| `created_at` | `TIMESTAMP` | нет | `NOW()` | Время создания |

Ограничения и индексы:

- `hypothesis_id` ссылается на `hypothesis.id` с `ON DELETE CASCADE`;
- `s3_key` уникален;
- для одной гипотезы допускается не более одного файла с `type = 'brandbook'`;
- индекс `file_hypothesis_id_idx` по `hypothesis_id`.

## `ad_campaign`

Созданная внешняя рекламная кампания.

| Поле | Тип | Nullable | Default | Описание |
|---|---|---:|---|---|
| `id` | `TEXT` | нет | — | Primary key |
| `provider` | `TEXT` | нет | — | Сейчас только `yandex` |
| `external_id` | `BIGINT` | нет | — | Идентификатор кампании у провайдера |
| `created_at` | `TIMESTAMP` | нет | `NOW()` | Время создания записи |
| `product_id` | `TEXT` | нет | — | Ссылка на `product.id` |

Ограничения и индексы:

- `product_id` ссылается на `product.id` с `ON DELETE CASCADE`;
- `provider` ограничен значением `yandex`;
- комбинация `(product_id, provider)` уникальна.

## Журнал запуска гипотезы

### `hypothesis_launch`

Попытка запуска рекламной гипотезы через Temporal workflow.

| Поле | Тип | Nullable | Default | Описание |
|---|---|---:|---|---|
| `id` | `TEXT` | нет | — | Primary key |
| `hypothesis_id` | `TEXT` | нет | — | Ссылка на `hypothesis.id` |
| `user_id` | `TEXT` | нет | — | Ссылка на `users.id` |
| `product_id` | `TEXT` | нет | — | Ссылка на `product.id` |
| `workflow_id` | `TEXT` | нет | — | Уникальный идентификатор Temporal workflow |
| `status` | `TEXT` | нет | — | `queued`, `running`, `succeeded`, `failed` или `reconciliation_required` |
| `last_error` | `TEXT` | да | — | Последняя ошибка запуска |
| `created_at` | `TIMESTAMP` | нет | `NOW()` | Время создания запуска |
| `completed_at` | `TIMESTAMP` | да | — | Время завершения |

Все три ссылки (`hypothesis_id`, `user_id`, `product_id`) используют
`ON DELETE CASCADE`. `workflow_id` уникален.

### `hypothesis_launch_resource`

Ресурс рекламной кампании, создаваемый в рамках запуска.

| Поле | Тип | Nullable | Default | Описание |
|---|---|---:|---|---|
| `id` | `TEXT` | нет | — | Primary key |
| `launch_id` | `TEXT` | нет | — | Ссылка на `hypothesis_launch.id` |
| `kind` | `TEXT` | нет | — | `campaign`, `retargeting_list`, `ad_group` или `audience_target` |
| `logical_key` | `TEXT` | нет | — | Стабильный логический ключ ресурса |
| `position` | `INTEGER` | нет | — | Позиция ресурса |
| `name` | `TEXT` | да | — | Название |
| `gender` | `TEXT` | да | — | Пол аудитории |
| `age` | `INTEGER` | да | — | Возраст |
| `start_date` | `TEXT` | да | — | Дата начала в строковом формате |
| `external_id` | `BIGINT` | да | — | Идентификатор ресурса у провайдера |
| `status` | `TEXT` | нет | — | `pending`, `creating`, `created` или `deleted` |

`launch_id` ссылается на `hypothesis_launch.id` с `ON DELETE CASCADE`.
Комбинация `(launch_id, kind, logical_key)` уникальна.

## Outbox для Temporal

Outbox-таблицы обеспечивают доставку команд в Temporal после фиксации
изменений backend в PostgreSQL.

### `temporal_outbox`

| Поле | Тип | Nullable | Default | Описание |
|---|---|---:|---|---|
| `id` | `TEXT` | нет | — | Primary key |
| `launch_id` | `TEXT` | нет | — | Уникальная ссылка на `hypothesis_launch.id` |
| `status` | `TEXT` | нет | — | `pending`, `starting` или `dispatched` |
| `attempts` | `INTEGER` | нет | `0` | Число попыток доставки |
| `available_at` | `TIMESTAMP` | нет | `NOW()` | Время, после которого запись можно обработать |
| `updated_at` | `TIMESTAMP` | нет | `NOW()` | Время последнего изменения |
| `last_error` | `TEXT` | да | — | Последняя ошибка доставки |

`launch_id` уникален и ссылается на `hypothesis_launch.id` с `ON DELETE CASCADE`.
Для поиска ожидающих записей используется частичный индекс
`temporal_outbox_pending_idx` по `available_at`, где `status = 'pending'`.

### `temporal_stop_outbox`

Имеет ту же структуру и статусы, что `temporal_outbox`, но относится к
остановке гипотезы.

| Поле | Тип | Nullable | Default | Описание |
|---|---|---:|---|---|
| `id` | `TEXT` | нет | — | Primary key |
| `stop_id` | `TEXT` | нет | — | Уникальная ссылка на `hypothesis_stop.id` |
| `status` | `TEXT` | нет | — | `pending`, `starting` или `dispatched` |
| `attempts` | `INTEGER` | нет | `0` | Число попыток доставки |
| `available_at` | `TIMESTAMP` | нет | `NOW()` | Время, после которого запись можно обработать |
| `updated_at` | `TIMESTAMP` | нет | `NOW()` | Время последнего изменения |
| `last_error` | `TEXT` | да | — | Последняя ошибка доставки |

`stop_id` уникален и ссылается на `hypothesis_stop.id` с `ON DELETE CASCADE`.
Для поиска ожидающих записей используется частичный индекс
`temporal_stop_outbox_pending_idx` по `available_at`, где `status = 'pending'`.

## Журнал остановки гипотезы

### `hypothesis_stop`

Попытка остановить ранее запущенную гипотезу через Temporal workflow.

| Поле | Тип | Nullable | Default | Описание |
|---|---|---:|---|---|
| `id` | `TEXT` | нет | — | Primary key |
| `hypothesis_id` | `TEXT` | нет | — | Ссылка на `hypothesis.id` |
| `user_id` | `TEXT` | нет | — | Ссылка на `users.id` |
| `source_launch_id` | `TEXT` | нет | — | Запуск, который останавливается |
| `workflow_id` | `TEXT` | нет | — | Уникальный идентификатор Temporal workflow |
| `status` | `TEXT` | нет | — | `queued`, `running`, `succeeded` или `requires_manual_intervention` |
| `last_error` | `TEXT` | да | — | Последняя ошибка остановки |
| `created_at` | `TIMESTAMP` | нет | `NOW()` | Время создания остановки |
| `completed_at` | `TIMESTAMP` | да | — | Время завершения |

`hypothesis_id`, `user_id` и `source_launch_id` используют `ON DELETE CASCADE`.
`workflow_id` уникален.

## `product_feedback_survey_result`

Последний сохранённый результат опроса обратной связи по персоне.

| Поле | Тип | Nullable | Default | Описание |
|---|---|---:|---|---|
| `persona_id` | `TEXT` | нет | — | Primary key персоны |
| `persona_name` | `TEXT` | нет | — | Имя персоны |
| `response` | `JSONB` | нет | — | Ответ опроса |
| `updated_at` | `TIMESTAMP` | нет | `NOW()` | Время последнего обновления |

## Жизненный цикл данных

1. Пользователь создаёт `hypothesis`, затем связанный с ней `product`.
2. Generator получает данные продукта через отдельную очередь и возвращает
   результаты в `audience`; связь с задачей хранится в `generator_job_id`.
3. Файлы гипотезы регистрируются в `file`, а бинарное содержимое хранится вне
   PostgreSQL.
4. Запуск создаёт `hypothesis_launch`, его дочерние
   `hypothesis_launch_resource` и запись `temporal_outbox`.
5. После постановки команды в Temporal outbox переводится в `dispatched`, а
   статус запуска обновляется workflow.
6. Остановка создаёт `hypothesis_stop` и связанную запись
   `temporal_stop_outbox`.
7. Удаление пользователя или гипотезы каскадно удаляет связанные доменные
   записи согласно foreign key.
