Yandex общая информация:
  yandex-direct-docs/docs/un...  @GitHub

VK ads API (возмжно не полное):

  GitHub - atomachinskiy/cla...  @GitHub

оффициальное VK API:

  https://ads.vk.ru/help/features/help_api


yandex API:
  GitHub - DragonSigh/yandex...  @GitHub (возмжно устаревшее) 10 м назад
  GitHub - biplane/yandex-di...  @GitHub 2 мес назад

# vk
Пример запроса для рекламной кампании по сообществу ВКонтакте с целевым действием "Вступить в сообщество" / "Написать в сообщество":

```http
POST /api/v2/ad_groups/{{ad_group_id}}/banners.json
Host: ads.vk.com
Authorization: Bearer {{access_token}}
```

```json
{
  "name": "Объявление 2023-09-01",
  "urls": {
    "primary": {
      "id": 12345
    }
  },
  "content": {
    "icon_256x256": {
      "id": 20414
    },
    "image_1080x607": {
      "id": 20416
    }
  },
  "textblocks": {
    "text_2000": {
      "text": "Сумки и аксессуары из натуральной кожи, созданные специально для Вас!"
    },
    "cta_community_vk": {
      "text": "signUp"
    },
    "about_company_115": {
      "text": "Ситникова Елена Юрьевна, ИНН 250300980000"
    },
    "title_40_vkads": {
      "text": "Сумки из натуральной кожи"
    }
  }
}
```

быстрый старт: https://ads.vk.ru/doc/api/info/%D0%91%D1%8B%D1%81%D1%82%D1%80%D1%8B%D0%B9%20%D1%81%D1%82%D0%B0%D1%80%D1%82

пример создания компании:

POST /api/v2/ad_plans.json
Host: ads.vk.com
Authorization: Bearer {ACCESS_TOKEN}

{
    "name": "Рекламная кампания 2023-09-20",
    "status": "active",
    "date_start": "2023-09-20",
    "date_end": "2023-09-25",
    "autobidding_mode": "max_goals",
    "budget_limit_day": 1000,
    "budget_limit": null,
    "max_price": 0,
    "objective": "socialengagement",
    "ad_object_id": 61732652,
    "ad_object_type": "url",
    "ad_groups": [
        {
            "name": "Группа объявлений 2023-09-20",
            "targetings": {
                "geo": {
                    "regions": [
                        5506
                    ]
                },
                "sex": [
                    "female"
                ],
                "age": {
                    "age_list": [
                        0,
                        19,
                        20,
                        21,
                        22
                    ]
                }
            },
            "max_price": 0,
            "budget_limit": null,
            "budget_limit_day": 1000,
            "date_start": "2023-09-20",
            "date_end": null,
            "age_restrictions": "0+",
            "package_id": 3122,
            "banners": [
                {
                    "name": "Объявление 2023-09-20",
                    "urls": {
                        "primary": {
                        "id": 61732652
                        }
                    },
                    "content": {
                        "icon_256x256": {
                            "id": 21506470
                        },
                        "image_1080x607": {
                            "id": 21507733
                        }
                    },
                    "textblocks": {
                        "text_2000": {
                            "text": "Здесь описание баннера"
                        },
                        "cta_community_vk": {
                            "text": "signUp"
                        },
                        "about_company_115": {
                            "text": "Здесь информация о рекламодателе"
                        },
                        "title_40_vkads": {
                            "text": "Здесь заголовок баннера"
                        }
                    }
                }
            ]
        }
    ]
}

package_id:
3122 - Вступить в сообщество
3127 - Написать в сообщество
3194 - Повысить вовлеченность

Верхнеуровневый объект - "Рекламная кампания" (ad_plan)
ad_object_id - id рекламируемого объекта
autobidding_mode - оптимизация бюджета кампании
max_price - верхняя граница для автоматического регулирования цены в валютных рублях (0 - автоматическое управление ставкой). См. также Гайд по автоматическому управлению ставкой
ad_groups - список объектов "Группа объявлений" (ad_group)
budget_limit - общий бюджет кампании в валютных рублях
budget_limit_day - дневной бюджет кампании в валютных рублях
date_start - дата начала трансляции кампании
date_end - дата окончания трансляции кампании (может быть null)
objective - целевое действие рекламной кампании

Объект "Группа объявлений" (ad_group)
age_restrictions - возрастная маркировка (0+/6+/12+/16+/18+)
banners - список объектов "объявление" (banner)
budget_limit - общий бюджет группы в валютных рублях
budget_limit_day - дневной бюджет группы в валютных рублях
date_start - дата начала трансляции группы (не может быть раньше даты начала трансляции кампании)
date_end - дата окончания трансляции группы (может быть null, тогда она будет унаследована от даты окончания трансляции кампании)
max_price - верхняя граница для автоматического регулирования цены в валютных рублях (0 - автоматическое управление ставкой). См. также Гайд по автоматическому управлению ставкой
package_id - идентификатор рекламного пакета, применяемого для данной рекламной кампании
targetings - таргетинги, настройки группы объявлений, которые позволяют выбрать целевую аудиторию. Подробнее о таргетингах ниже.

Объект "Объявление" (banner)
urls - ссылки
content - креативы (статические изображения, видео и т.п.)
textblocks - текстовые блоки объявления
у каждой кампании должен быть пакет

https://ads.vk.ru/doc/api/object/Package

name
objective
url_types
prices

Объект, предоставляющий информацию о пакете рекламных услуг.

Используется в методах: **Packages**

| Field name | Type | Conditions | Description |
|---|---|---|---|
| `banner_url_get_params` | string | `readable` | GET-параметры, добавляемые к URL каждого объявления |
| `created` | datetime | `readable` `default_field` | Время создания пакета |
| `description` | string | `readable` `default_field` | Описание пакета |
| `id` | id | `readable` `default_field` `min_value=1` `max_value=2147483647` | Идентификатор пакета |
| `max_price_per_unit` | priceperunit | `readable` `default_field` | Максимальная стоимость кампании, которую можно установить в рамках выбранного пакета |
| `max_uniq_shows_limit` | integer | `readable` `default_field` `max_value=2147483647` | Максимальное количество показов объявлений одному пользователю, которое можно установить в рамках выбранного пакета |
| `name` | string | `readable` `default_field` | Название пакета |
| `objective` | list of string | `readable` `default_field` | Возможные цели рекламных кампаний в данном пакете. Допустимые значения: `reach` — Охват; `traffic` — Трафик; `appinstalls` — Установки приложений; `reengagement` — Ремаркетинг в приложение; `playersengagement` — Привлечение игроков игр в соц. сетей; `videoviews` — Просмотр видео; `storeproductsales` — Покупки в интернет-магазине; `engagement` — Конверсии; `articleviews` — Просмотр статей; `audiolistening` — Аудиореклама; `socialengagement` — Действия в социальных сетях; `storevisits` — Посещение точек продаж; `premium_reach` — Охват в премиальной сети; `general_ttm` — Медийные размещения «Продукты Mail.Ru Group» |
| `options` | object | `readable` | Списки доступных таргетингов (targetings) и настроек (settings) рекламной кампании, созданной на основе пакета. Могут содержать поля `"defaults"` и `"values"`, хранящие таргетинговые значения по умолчанию и список всех возможных для использования значений соответственно. Расширенная информация о площадках, на которые можно настроить таргетинг, доступна с помощью метода `PackagesPads` |
| `package_request` | string | `readable` | Статус доступности пакета пользователю: `allowed` — пакет доступен; `requested` — пакет запрошен, но еще недоступен; `can_be_requested` — пакет может быть запрошен пользователем в службе поддержки VK Ads: [support_target@corp.my.com](mailto:support_target@corp.my.com) |
| `pads_tree_id` | id | `readable` `default_field` `min_value=1` `max_value=2147483647` | Идентификатор дерева площадок. Имеет значение 0, если дерево плоское |
| `paid_event_type` | integer | `readable` `default_field` `min_value=-2147483647` `max_value=2147483647` | Идентификатор события, за которое производится оплата: 0 — 1000 показов; 1 — клик; 7 — конверсия; 1013 — просмотр всего видео; 1017 — просмотр 10 секунд видео |
| `price` | priceperunit | `readable` `default_field` | Стоимость рекламной кампании в выбранном пакете (значение по умолчанию). Данное значение устанавливается, если при создании кампании не была указана другая цена |
| `priced_event_type` | integer | `readable` `default_field` `min_value=-2147483647` `max_value=2147483647` | Идентификатор события, по которому оптимизируется открутка кампании: 0 — 1000 показов; 1 — клик; 7 — конверсия; 30 — установки; 41 — события в Сообществах ВКонтакте; 43 — in-app события в VK Mini Apps; 51 — лидформы; 1013 — просмотр всего видео; 1017 — просмотр 10 секунд видео |
| `related_package_ids` | list of type | `readable` `default_field` | Идентификаторы пакетов, на которые можно переключать уже созданные рекламные кампании текущего пакета |
| `status` | string | `readable` `writable` `default_field` `choices=`<br><br>- `active` — Активный пакет<br>- `deleted` — Удаленный пакет<br>- `blocked` — Заблокированный пакет | Статус пакета. Создавать кампании можно с использованием только активного пакета |
| `updated` | datetime | `readable` `default_field` | Время последнего обновления пакета |
| `url_types` | object | `readable` `default_field` | Типы ссылок, разрешенные в пакете. Представляет из себя объект, в котором в качестве ключа идет роль ссылки, в качестве значения — массив массивов строк. Массивы второго уровня содержат список типов ссылок. Ссылка должна иметь все указанные типы для того, чтобы быть разрешенной в этой роли. Массивы первого уровня объединены по принципу ИЛИ, то есть достаточно чтобы ссылка соответствовала одному из перечислений, указанных в одном из массивов второго уровня. К примеру, если поле имеет значение `[[type_1, type_2], [type_1, type_3]]`, то ссылка должна иметь типы `type_1` и `type_2` или `type_1` и `type_3` |

# yandex
https://yandex.com/dev/direct/doc/ru/objects/objects

flowchart TB
    Campaign["Campaign"]
    AdGroup["AdGroup"]
    Ad["Ad"]
    AdImage["AdImage"]
    VCard["VCard"]
    SitelinkSet["SitelinkSet"]
    AdExtension["AdExtension"]
    Keyword["Keyword"]
    AudienceTarget["AudienceTarget"]
    RetargetingList["RetargetingList"]

    Campaign -->|"1 : 0..1000"| AdGroup
    AdGroup -->|"1 : 0..50"| Ad
    Ad -->|"0..* : 0..1"| AdImage
    Ad -->|"0..* : 0..1"| VCard
    Ad -->|"0..* : 0..1"| SitelinkSet
    Ad -->|"0..* : 0..50"| AdExtension
    AdGroup -->|"1 : 0..200"| Keyword
    AdGroup -->|"1 : 0..50"| AudienceTarget
    AudienceTarget -->|"0..* : 1"| RetargetingList

обзор api
https://yandex.com/dev/direct/doc/ru/concepts/overview

справичник всех операций
https://yandex.ru/dev/direct/doc/ru/concepts/about

виды объявлений
- TextAd — текстово-графическое. Заголовки (Title, Title2), текст, ссылка, изображение, визитка, быстрые ссылки, видеодополнение, цена. С 30 июня доступно только для редактирования.
- TextImageAd — графическое на основе одного изображения (FIXED_IMAGE) + ссылка/Турбо-страница.
- TextAdBuilderAd — графическое из креатива конструктора креативов + ссылка/Турбо-страница.
- CpmBannerAdBuilderAd — медийное (баннер) из креатива, с трекинг-пикселями (Аудитории/ADFOX).
- CpmVideoAdBuilderAd — медийное видеообъявление (в медийной кампании) из креатива с пикселями ADFOX.
- ResponsiveAd — комбинаторное: наборы заголовков (1–7) и текстов (1–3), до 5 изображений, до 6 видео, цена, расширения. Приоритетный тип вместо устаревшего TextAd.
- ShoppingAd — товарное, генерируется из фида (FeedId) с фильтрами предложений, заголовками/текстами из полей фида и текстами по умолчанию.
- ListingAd — для страниц каталога, структура аналогична ShoppingAd (фид + фильтры + источники полей), но предназначено для каталогов, а не товарных предложений.

campaign:
```
{
  "method": "add",
  "params": { /* params */
    "Campaigns": [{  /* CampaignAddItem */
      "ClientInfo": (string), Название клиента (до 255 символов).
      "TimeZone": (string), Часовой пояс в месте нахождения пользователя Значение по умолчанию Europe/Moscow.
      "Name": (string), /* required */ Название кампании (до 255 символов).
      "StartDate": (string), /* required */ Дата начала показов объявлений в формате YYYY-MM-DD. Должна быть не меньше текущей даты.
      "DailyBudget": {  /* DailyBudget */
        "Amount": (long), /* required */
        "Mode": ( "STANDARD" | "DISTRIBUTED" ) /* required */
      },
      "EndDate": (string), Дата окончания показов объявлений в формате YYYY-MM-DD.
      "NegativeKeywords": {  /* ArrayOfString */ Массив минус-фраз, общих для всех ключевых фраз кампании. 
        "Items": [(string), ... ] /* required */
      },
      "BlockedIps": {  /* ArrayOfString */ Массив IP-адресов, которым не нужно показывать объявления
        "Items": [(string), ... ] /* required */
      },
      "ExcludedSites": {  /* ArrayOfString */
        "Items": [(string), ... ] /* required */
      },
      "TextCampaign": {  /* TextCampaignAddItem */ Настройки для кампании с типом «Текстово-графические объявления».
        ... text campaign params ... https://yandex.ru/dev/direct/doc/ru/campaigns/add-text-campaign
      },
      "MobileAppCampaign": {  /* MobileAppCampaignAddItem */
        ... mobile app campaign params ...
      },
      "CpmBannerCampaign": {  /* CpmBannerCampaignAddItem */
        ... cpm banner campaign params ...
      },
      "UnifiedCampaign": {  /* UnifiedCampaignAddItem */
        ... unified campaign params ...
      },
      "TimeTargeting": {  /* TimeTargetingAdd */
        "Schedule": {  /* ArrayOfString */
          "Items": [(string), ... ] /* required */
        },
        "ConsiderWorkingWeekends": ( "YES" | "NO" ), /* required */
        "HolidaysSchedule": {  /* TimeTargetingOnPublicHolidays */
          "SuspendOnHolidays": ( "YES" | "NO" ), /* required */
          "BidPercent": (int),
          "StartHour": (int),
          "EndHour": (int)
        }
      }
    }, ...] /* required */
  }
}

```

ad group:
```
{
  "method": "add",
  "params": { /* params */
    "AdGroups": [{  /* AdGroupAddItem */
      "Name": (string), /* required */
      "CampaignId": (long), /* required */
      "RegionIds": [(long), ... ], /* required */
      "NegativeKeywords": { /* ArrayOfString */
        "Items": [(string), ... ] /* required */
      },
      "NegativeKeywordSharedSetIds": { /* ArrayOfLong */
        "Items": [(long), ... ] /* required */
      },
      "TrackingParams": (string),
      "MobileAppAdGroup": {  /* MobileAppAdGroupAdd */
        "StoreUrl": (string), /* required */
        "TargetDeviceType": [( "DEVICE_TYPE_MOBILE" | "DEVICE_TYPE_TABLET" ), ... ], /* required */
        "TargetCarrier": ( "WI_FI_ONLY" | "WI_FI_AND_CELLULAR" ), /* required */
        "TargetOperatingSystemVersion": (string) /* required */
      },
      "DynamicTextAdGroup": [{  /* DynamicTextAdGroupAdd */
        "DomainUrl": (string) /* required */,
        "AutotargetingCategories" : [{  /* AutotargetingCategoriesAdd */
          "Category" : ( "EXACT" | "ALTERNATIVE" | "COMPETITOR" | "BROADER" | "ACCESSORY" ) /* required */,
          "Value" : ( "YES" | "NO" ) /* required */
        }, ...]
      }, ...],
      "DynamicTextFeedAdGroup": [{  /* DynamicTextFeedAdGroupAdd */
        "FeedId": (long) /* required */,
        "AutotargetingCategories" : [{  /* AutotargetingCategoriesAdd */
          "Category" : ( "EXACT" | "ALTERNATIVE" | "COMPETITOR" | "BROADER" | "ACCESSORY" ) /* required */,
          "Value" : ( "YES" | "NO" ) /* required */
        }, ...]
      }, ...],
      "CpmBannerKeywordsAdGroup": {  /* CpmBannerKeywordsAdGroupAdd */
      },
      "CpmBannerUserProfileAdGroup": {  /* CpmBannerUserProfileAdGroupAdd */
      },
      "CpmVideoAdGroup": {  /* CpmVideoAdGroupAdd */
      },
      "SmartAdGroup": {  /* SmartAdGroupAdd */
        "FeedId": (long), /* required */
        "AdTitleSource": (string),
        "AdBodySource": (string)
      },
      "UnifiedAdGroup" : {
        "OfferRetargeting" : ("YES"|"NO") /* required */
      },
      "TextAdGroupFeedParams" : {  /*TextAdGroupFeedParamsAdd */
        "FeedId" : (long) /* required */,
        "FeedCategoryIds" : {
          "Items" : [ (long) ] /* required */
        }
      }
    }, ... ] /* required */
  }
}
```

ad
```
https://yandex.ru/dev/direct/doc/ru/ads/add

{
  "method": "add",
  "params": { /* params */
    "Ads": [{  /* AdAddItem */
      "AdGroupId": (long), /* required */
      "TextAd": {  /* TextAdAdd */           кажется нельзя больше добавлять такие
        "Title": (string), /* required */
        "Title2": (string),
        "Text": (string), /* required */
        "Href": (string),
        "Mobile": ( "YES" | "NO" ), /* required */
        "DisplayUrlPath": (string),
        "VCardId": (long),
        "AdImageHash": (string),
        "SitelinkSetId": (long),
        "AdExtensionIds": [(long), ... ],
        "VideoExtension": {  /* VideoExtensionAddItem */
          "CreativeId": (long)
        },
        "PriceExtension": {  /* PriceExtensionAddItem */
          "Price": (long), /* required */
          "OldPrice": (long),
          "PriceQualifier": ( "FROM" | "UP_TO" | "NONE" ), /* required */
          "PriceCurrency": ( "RUB" | "BYN" | "CHF" | "EUR" | "KZT" | "TRY" | "UAH" | "USD" | "UZS" ) /* required */
        },
        "TurboPageId": (long),
        "BusinessId": (long),
        "PreferVCardOverBusiness": ( "YES" | "NO" ),
        "ErirAdDescription" : (string)
      },
      "TextImageAd": {  /* TextImageAdAdd */
        "AdImageHash": (string), /* required */  Хэш изображения. Для графических объявлений подходят только изображения с типом FIXED_IMAGE,
        "ErirAdDescription" : (string), Описание объекта продвижения.
        "Href": (string),
        "TurboPageId": (long)
      },
      "TextAdBuilderAd": {  /* TextAdBuilderAdAdd */
        "Creative": { /* AdBuilderAdAddItem */
          "CreativeId": (long) /* required */
        }, /* required */
        "ErirAdDescription" : (string),
        "Href": (string),
        "TurboPageId": (long)
      },
      "CpmBannerAdBuilderAd": {  /* CpmBannerAdBuilderAdAdd */
        "Creative": { /* AdBuilderAdAddItem */
          "CreativeId": (long) /* required */
        }, /* required */
        "ErirAdDescription" : (string),
        "Href": (string),
        "TrackingPixels": { /* ArrayOfString */
          "Items": [(string), ... ] /* required */
        },
        "TurboPageId": (long)
      },
      "CpmVideoAdBuilderAd": {  /* CpmVideoAdBuilderAdAdd */
        "Creative": { /* AdBuilderAdAddItem */
          "CreativeId": (long) /* required */
        }, /* required */
        "ErirAdDescription" : (string),
        "Href": (string),
        "TrackingPixels": { /* ArrayOfString */
          "Items": [(string), ... ] /* required */
        },
        "TurboPageId": (long)
      },
      "ShoppingAd" : {
        "SitelinkSetId" : (long),
        "AdExtensionIds" : [ (long) ],
        "BusinessId" : (long),
        "FeedId" : (long) /* required */,
        "FeedFilterConditions" : [ {
          "Operand" : (string) /* required */,
          "Operator" : ("CONTAINS_ANY"|"EQUALS_ANY"|"EXISTS"|"GREATER_THAN"|"IN_RANGE"|"LESS_THAN"|"NOT_CONTAINS_ALL") /* required */,
          "Arguments" : [ (string) ] /* required */
        }, ... ],
        "TitleSources" : [ (string) ],
        "TextSources" : [ (string) ],
        "DefaultTexts" : [ (string) ] /* required */
      },
      "ListingAd" : {
        "SitelinkSetId" : (long),
        "AdExtensionIds" : [ (long) ],
        "BusinessId" : (long),
        "FeedId" : (long) /* required */,
        "FeedFilterConditions" : [ {
          "Operand" : (string) /* required */,
          "Operator" : ("CONTAINS_ANY"|"EQUALS_ANY"|"EXISTS"|"GREATER_THAN"|"IN_RANGE"|"LESS_THAN"|"NOT_CONTAINS_ALL") /* required */,
          "Arguments" : [ (string) ] /* required */
        }, ... ],
        "TitleSources" : [ (string) ],
        "TextSources" : [ (string) ],
        "DefaultTexts" : [ (string) ] /* required */
      },
      "ResponsiveAd": {  /* ResponsiveAdAdd */
        "Titles": [(string), ...], /* required */ Набор заголовков. Можно добавить от 1 до 7 заголовков.
        "Texts": [(string), ...], /* required */ Набор текстов объявления. Можно добавить от 1 до 3 текстов.
        "Href": (string), Ссылка на сайт, куда будет вести объявление
        "AgeLabel": ( "AGE_0" | ... | "MONTHS_12" ), Возрастная категория.
        "DisplayUrlPath": (string), Отображаемая ссылка Допускается только при наличии параметра Href
        "AdImageHashes": [(string), ...], Хэши изображений. Можно указать от 1 до 5 изображений.
        "SitelinkSetId": (long),
        "AdExtensionIds": [(long), ... ], Идентификаторы в идеодополнений
        "VideoExtensionIds": [(long), ... ],
        "PriceExtension": {  /* PriceExtensionAddItem */ Цена в объявлении.
          "Price": (long), /* required */
          "OldPrice": (long),
          "PriceQualifier": ( "FROM" | "UP_TO" | "NONE" ), /* required */
          "PriceCurrency": ( "RUB" | "BYN" | "CHF" | "EUR" | "KZT" | "TRY" | "UAH" | "USD" | "UZS" ) /* required */
        },
        "BusinessId": (long),
        "ErirAdDescription" : (string)
      }
    }, ... ] /* required */
  }
}
```

ad image:
```
{
  "method": "add",
  "params": { /* params */
    "AdImages": [{  /* AdImageAddItem */
      "ImageData": (base64Binary), /* required */
      "Type" : ("REGULAR" | "WIDE" | "FIXED_IMAGE" | "AUTO" ),
      "Name": (string) /* required */
    }, ... ] /* required */
  }
}
"ResponsiveAd": {  /* ResponsiveAdAdd */
  "Titles": [(string), ...], /* required */
  "Texts": [(string), ...], /* required */
  "Href": (string),
  "AgeLabel": ( "AGE_0" | ... | "MONTHS_12" ),
  "DisplayUrlPath": (string),
  "AdImageHashes": [(string), ...],
  "SitelinkSetId": (long),
  "AdExtensionIds": [(long), ... ],
  "VideoExtensionIds": [(long), ... ],
  "PriceExtension": {  /* PriceExtensionAddItem */
    "Price": (long), /* required */
    "OldPrice": (long),
    "PriceQualifier": ( "FROM" | "UP_TO" | "NONE" ), /* required */
    "PriceCurrency": ( "RUB" | "BYN" | "CHF" | "EUR" | "KZT" | "TRY" | "UAH" | "USD" | "UZS" ) /* required */
  },
  "BusinessId": (long),
  "ErirAdDescription" : (string)
}
```

# avito:
https://developers.avito.ru/api-catalog/ads/documentation#info/koncepcii_i_ierarhiya_sushchnostey

## Концепции и иерархия сущностей

Сущности API связаны иерархически:

```text
Аккаунт (Account)
├── Рекламодатели (Advertiser)  ← юр., от лица которого размещается реклама
│   └── Договоры (Contract)     ← нужен, если рекламодатель ≠ аккаунт
├── Дочерние аккаунты (Child)  ← отдельные аккаунты на одном договоре с родителем
└── Кампании (Campaign)
    └── Группы (Group)          ← бюджет, ставка, таргетинг, расписание
        └── Креативы (Creative) ← конкретные объявления
```

| Сущность | Что это |
|---|---|
| **Аккаунт** | Рекламный кабинет конкретного ЮЛ/ИП. Токен API всегда привязан к одному аккаунту. Подробнее ↗ |
| **Рекламодатель** | Юридическое лицо (или ИП), от лица которого размещается реклама. У одного аккаунта может быть несколько рекламодателей. Подробнее ↗ |
| **Договор** | Изначальный договор между аккаунтом и рекламодателем. Нужен, только если рекламодатель и аккаунт — разные ЮЛ/ИП. Подробнее ↗ |
| **Дочерний аккаунт** | Отдельный аккаунт на одном договоре с родителем. У него собственный токен и баланс; деньги/бонусы переводятся с родителя. Подробнее ↗ |
| **Кампания** | Набор рекламных настроек с общей идеей и целью, моделью оплаты (`CPM` / `CPC`) и типом (`textImage` / `HTML` / `video`). Подробнее ↗ |
| **Группа** | Внутри кампании; задаёт бюджет, ставку, расписание и таргетинг. Подробнее ↗ |
| **Креатив** | Конкретное объявление (баннер/видео/HTML) внутри группы. Подробнее ↗ |

> Через текущую версию API создавать кампании, группы и креативы пока нельзя — это делается в веб-интерфейсе.

```
https://developers.avito.ru/api-catalog/ads/documentation#operation/V1GetCampaignsList

"Campaign": {
"properties": {
  "accountId": {
    "description": "ID аккаунта",
    "type": "integer"
  },
  "additionalAgreementID": {
    "description": "ID дополнительного соглашения к изначальному договору",
    "nullable": true,
    "type": "integer"
  },
  "advertiserId": {
    "description": "ID рекламодателя",
    "type": "integer"
  },
  "budget": {
    "description": "Бюджет кампании в рублях с НДС",
    "type": "integer"
  },
  "budgetKopeks": {
    "description": "Бюджет кампании в копейках с НДС",
    "type": "integer"
  },
  "campaignType": {
    "$ref": "#/components/schemas/CampaignType"
  },
  "contractId": {
    "description": "ID контракта",
    "type": "integer"
  },
  "createdAt": {
    "description": "Дата создания кампании",
    "type": "string"
  },
  "endDate": {
    "description": "Дата окончания кампании",
    "nullable": true,
    "type": "string"
  },
  "id": {
    "description": "ID кампании",
    "type": "integer"
  },
  "managerID": {
    "description": "ID менеджера, ответственного за кампанию (userID)",
    "type": "integer"
  },
  "name": {
    "description": "Название кампании",
    "type": "string"
  },
  "paymentModel": {
    "$ref": "#/components/schemas/CampaignPaymentModel"
  },
  "performanceAdditionalAgreementID": {
    "description": "ID доп соглашения к исполнительскому договору",
    "nullable": true,
    "type": "integer"
  },
  "startDate": {
    "description": "Дата начала кампании",
    "nullable": true,
    "type": "string"
  },
  "status": {
    "$ref": "#/components/schemas/CampaignStatus"
  },
  "updatedAt": {
    "description": "Дата обновления кампании",
    "nullable": true,
    "type": "string"
  },
  "userId": {
    "description": "ID пользователя создавшего кампанию",
    "type": "integer"
  }
},
https://developers.avito.ru/api-catalog/ads/documentation#operation/V1GetGroupsList

"Group": {
  "properties": {
    "accountID": {
      "description": "ID аккаунта",
      "type": "integer"
    },
    "actions": {
      "description": "Доступные действия с группой",
      "items": {
        "type": "string"
      },
      "type": "array"
    },
    "activitySchedule": {
      "description": "Расписание активности по дням",
      "properties": {
        "friday": {
          "$ref": "#/components/schemas/ActivityScheduleHours"
        },
        "monday": {
          "$ref": "#/components/schemas/ActivityScheduleHours"
        },
        "saturday": {
          "$ref": "#/components/schemas/ActivityScheduleHours"
        },
        "sunday": {
          "$ref": "#/components/schemas/ActivityScheduleHours"
        },
        "thursday": {
          "$ref": "#/components/schemas/ActivityScheduleHours"
        },
        "tuesday": {
          "$ref": "#/components/schemas/ActivityScheduleHours"
        },
        "wednesday": {
          "$ref": "#/components/schemas/ActivityScheduleHours"
        }
      },
      "type": "object"
    },
    "advertiserID": {
      "description": "ID рекламодателя",
      "type": "integer"
    },
    "audienceType": {
      "$ref": "#/components/schemas/AudienceType"
    },
    "budget": {
      "description": "Бюджет группы в рублях с НДС",
      "type": "integer"
    },
    "budgetFrequency": {
      "$ref": "#/components/schemas/BudgetFrequency"
    },
    "budgetKopeks": {
      "description": "Бюджет группы в копейках с НДС",
      "type": "integer"
    },
    "campaignID": {
      "description": "ID кампании",
      "type": "integer"
    },
    "campaignType": {
      "$ref": "#/components/schemas/CampaignType"
    },
    "createdAt": {
      "description": "Дата создания группы",
      "format": "date-time",
      "type": "string"
    },
    "format": {
      "$ref": "#/components/schemas/GroupFormat"
    },
    "frequencyCount": {
      "description": "Частота показов в рамках периода",
      "type": "integer"
    },
    "frequencyPeriod": {
      "$ref": "#/components/schemas/FrequencyPeriod"
    },
    "frequencyRules": {
      "description": "Правила частотного ограничения",
      "items": {
        "$ref": "#/components/schemas/FrequencyRule"
      },
      "type": "array"
    },
    "haveCreative": {
      "description": "Есть ли в группе креатив",
      "nullable": true,
      "type": "boolean"
    },
    "id": {
      "description": "ID группы",
      "type": "integer"
    },
    "managerID": {
      "description": "ID менеджера, ответственного за кампанию (userID)",
      "type": "integer"
    },
    "name": {
      "description": "Название группы",
      "type": "string"
    },
    "pace": {
      "$ref": "#/components/schemas/Paces"
    },
    "paymentModel": {
      "$ref": "#/components/schemas/CampaignPaymentModel"
    },
    "periodEndAt": {
      "description": "Конец периода кампании",
      "format": "date-time",
      "nullable": true,
      "type": "string"
    },
    "periodStartAt": {
      "description": "Начало периода кампании",
      "format": "date-time",
      "nullable": true,
      "type": "string"
    },
    "periodTimezone": {
      "description": "Таймзона периода показов",
      "type": "string"
    },
    "price": {
      "description": "Цена в рублях с НДС",
      "type": "integer"
    },
    "priceControl": {
      "$ref": "#/components/schemas/GroupPriceControl"
    },
    "priceKopeks": {
      "description": "Цена в копейках с НДС",
      "type": "integer"
    },
    "scheduleType": {
      "$ref": "#/components/schemas/ScheduleType"
    },
    "startASAP": {
      "description": "Запускать как можно скорее",
      "type": "boolean"
    },
    "status": {
      "$ref": "#/components/schemas/GroupsStatus"
    },
    "targetingAgeIDs": {
      "description": "ID возрастных сегментов",
      "items": {
        "type": "integer"
      },
      "type": "array"
    },
    "targetingAudienceIDs": {
      "description": "ID аудиторий",
      "items": {
        "type": "string"
      },
      "type": "array"
    },
    "targetingFamilyStatusIDs": {
      "description": "ID семейного статуса",
      "items": {
        "type": "integer"
      },
      "type": "array"
    },
    "targetingGenderIDs": {
      "description": "ID пола",
      "items": {
        "type": "integer"
      },
      "type": "array"
    },
    "targetingIncomeIDs": {
      "description": "ID доходных сегментов",
      "items": {
        "type": "integer"
      },
      "type": "array"
    },
    "targetingInterestIDs": {
      "description": "ID интересов",
      "items": {
        "type": "string"
      },
      "type": "array"
    },
    "targetingLocationIDs": {
      "description": "ID локаций",
      "items": {
        "type": "integer"
      },
      "type": "array"
    },
    "targetingParentStatusIDs": {
      "description": "ID родительского статуса",
      "items": {
        "type": "integer"
      },
      "type": "array"
    },
    "updatedAt": {
      "description": "Дата обновления группы",
      "format": "date-time",
      "type": "string"
    },
    "userID": {
      "description": "ID пользователя, создавшего группу",
      "type": "integer"
    }
  },
  "required": [
    "startASAP",
    "frequencyCount",
    "frequencyRules"
  ],
  "type": "object"
},
https://developers.avito.ru/api-catalog/ads/documentation#operation/V1GetCreativesList

"Creative": {
  "properties": {
    "accountID": {
      "description": "ID аккаунта",
      "type": "integer"
    },
    "advertiserID": {
      "description": "ID рекламодателя",
      "type": "integer"
    },
    "buttonText": {
      "description": "Текст на кнопке",
      "nullable": true,
      "type": "string"
    },
    "campaignID": {
      "description": "ID кампании",
      "type": "integer"
    },
    "campaignType": {
      "$ref": "#/components/schemas/CampaignType"
    },
    "description": {
      "description": "Описание объявления",
      "nullable": true,
      "type": "string"
    },
    "groupID": {
      "description": "ID группы",
      "type": "integer"
    },
    "id": {
      "description": "ID креатива",
      "type": "integer"
    },
    "landingID": {
      "description": "ID посадочной страницы",
      "nullable": true,
      "type": "integer"
    },
    "legalInfo": {
      "$ref": "#/components/schemas/LegalInfo"
    },
    "link": {
      "description": "Целевая ссылка",
      "type": "string"
    },
    "managerID": {
      "description": "ID менеджера, ответственного за кампанию (userID)",
      "type": "integer"
    },
    "name": {
      "description": "Название креатива",
      "type": "string"
    },
    "originUrl": {
      "description": "Оригинальная ссылка для аналитики",
      "nullable": true,
      "type": "string"
    },
    "paymentModel": {
      "$ref": "#/components/schemas/CampaignPaymentModel"
    },
    "previewUrl": {
      "description": "Ссылка на предпросмотр креатива",
      "nullable": true,
      "type": "string"
    },
    "status": {
      "$ref": "#/components/schemas/CreativesStatus"
    },
    "title": {
      "description": "Заголовок объявления",
      "nullable": true,
      "type": "string"
    },
    "userID": {
      "description": "ID пользователя, создавшего креатив",
      "type": "integer"
    }
  },
  "type": "object"
},
```
