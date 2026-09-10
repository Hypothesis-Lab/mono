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