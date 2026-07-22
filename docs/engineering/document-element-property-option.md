# Document Element Property Options

Dokumen ini merangkum flag property document builder yang disediakan dari seeder:

- `migration/document_properties.sql`
- `migration/document_property_options.sql`
- `migration/document_element_properties.sql`

Tujuannya agar frontend dapat melakukan sinkronisasi render, form editor, dan default value berdasarkan `code` property yang dikirim backend. Frontend sebaiknya menjadikan `code` sebagai key utama, sedangkan `token` hanya dipakai sebagai public identifier untuk komunikasi API.

## Sync Rules

- `property.code` adalah flag CSS/native rendering yang dipakai frontend, contoh `font-size`, `padding`, `grid-template-columns`.
- `property.input_type` menentukan tipe control editor di frontend.
- `property.data_type` menentukan cara parsing value sebelum dirender.
- `property.default_value` adalah fallback global property.
- `element_property.default_value` mengoverride default global untuk element tertentu.
- `property.options` hanya wajib dipakai ketika `input_type = select` atau ketika frontend ingin menampilkan preset.
- Value number dari backend tetap berbentuk string saat berada di JSON properties, lalu frontend dapat menambahkan unit berdasarkan `unit`.
- `unit = ""` berarti value tidak perlu unit, misalnya `font-weight`, `line-height`, `color`, atau keyword CSS.
- `grid-template-columns` memakai JSON array persen dalam bentuk string, contoh `[50,50]`. Total custom value harus dijaga frontend maksimum `100`.

## Data Type

| Data Type | Cara FE Membaca | Contoh |
| --- | --- | --- |
| `string` | Gunakan langsung sebagai CSS/string value | `left`, `Arial`, `#000000`, `100%` |
| `number` | Parse sebagai angka, lalu tambahkan `unit` jika ada | `16` + `px` |
| `boolean` | Parse sebagai boolean | `true`, `false` |
| `json` | Parse sebagai JSON | `[50,50]` |

## Input Type

| Input Type | Rekomendasi Control FE | Catatan |
| --- | --- | --- |
| `text` | Text input | Untuk value bebas seperti `auto`, `100%`, URL/string |
| `number` | Number input | Gunakan `unit` sebagai suffix visual |
| `select` | Select/dropdown | Opsi dari `document_property_options` |
| `switch` | Toggle | Saat ini belum ada seed aktif |
| `color` | Color picker | Value saat ini memakai format hex |
| `textarea` | Textarea | Saat ini belum ada seed aktif |
| `grid-columns` | Grid columns editor | Value berupa JSON array persen |

## Custom Size Values

`width` dan `height` sengaja memakai `data_type = string` dan `input_type = text`, sehingga frontend boleh menyediakan input custom, bukan hanya dropdown preset.

Contoh value yang valid untuk frontend renderer:

| Value | Meaning |
| --- | --- |
| `auto` | Mengikuti ukuran natural/content |
| `100%` | Mengikuti penuh parent |
| `50%` | Setengah parent |
| `320px` | Ukuran fixed pixel |
| `12rem` | Ukuran relatif root font |
| `fit-content` | Mengikuti content sesuai CSS native |
| `max-content` | Mengikuti ukuran maksimum content |

Options `width` dan `height` di seed hanya berfungsi sebagai preset cepat. Frontend tetap boleh mengirim custom value di `layer.properties.width` dan `layer.properties.height`.

## Document Settings

`documents.settings` dipakai untuk konfigurasi level dokumen dan region, bukan untuk styling element. Styling element tetap berada di `document_layers.properties`.

Default shape saat `settings` tidak dikirim:

```json
{
  "page": {
    "orientation": "portrait",
    "margin": {
      "top": 24,
      "right": 24,
      "bottom": 24,
      "left": 24,
      "unit": "px"
    }
  },
  "regions": {
    "header": {
      "height": 0,
      "unit": "px"
    },
    "body": {
      "height": null,
      "unit": "auto"
    },
    "footer": {
      "height": 0,
      "unit": "px"
    }
  }
}
```

- `document_layers.region` menentukan layer masuk ke `header`, `body`, atau `footer`.
- `documents.settings.regions.*.height` menentukan tinggi area region.
- `height = 0` dengan unit ukuran berarti region tidak dipakai.
- `height = null` dan `unit = auto` berarti tinggi mengikuti content/area render.
- Watermark tidak disimpan di `documents.settings`; gunakan element `watermark` pada `document_layers`.

## Element Master

| Code | Name | Renderer Tag | Content Type | Container |
| --- | --- | --- | --- | --- |
| `grid` | Grid | `div` | `none` | Yes |
| `text` | Text | `p` | `text` | No |
| `image` | Image | `img` | `image` | No |
| `watermark` | Watermark | `div` | `image` | No |
| `table` | Table | `table` | `table` | No |
| `divider` | Divider | `hr` | `none` | No |
| `spacer` | Spacer | `div` | `none` | No |

## Property Master

| Code | Name | Data Type | Input Type | Default | Unit | FE Usage |
| --- | --- | --- | --- | --- | --- | --- |
| `display` | Display | `string` | `select` | `inline` |  | CSS `display` |
| `grid-template-columns` | Grid Template Columns | `string` | `text` | `[50,50]` |  | CSS grid template columns/preset value |
| `padding` | Padding | `number` | `number` | `0` | `px` | CSS `padding` |
| `justify-items` | Justify Items | `string` | `select` | `legacy` |  | CSS `justify-items` |
| `align-items` | Align Items | `string` | `select` | `normal` |  | CSS `align-items` |
| `font-family` | Font Family | `string` | `select` | `initial` |  | CSS `font-family` |
| `font-size` | Font Size | `string` | `text` | `medium` |  | CSS `font-size` |
| `font-weight` | Font Weight | `string` | `select` | `400` |  | CSS `font-weight` |
| `font-style` | Font Style | `string` | `select` | `normal` |  | CSS `font-style` |
| `text-decoration` | Text Decoration | `string` | `select` | `none` |  | CSS `text-decoration` |
| `line-height` | Line Height | `string` | `text` | `normal` |  | CSS `line-height` |
| `color` | Color | `string` | `color` | `canvastext` |  | CSS `color` |
| `text-align` | Text Align | `string` | `select` | `start` |  | CSS `text-align` |
| `margin` | Margin | `number` | `number` | `0` | `px` | CSS `margin` |
| `width` | Width | `string` | `text` | `auto` |  | CSS `width` |
| `height` | Height | `string` | `text` | `auto` |  | CSS `height` |
| `object-fit` | Object Fit | `string` | `select` | `fill` |  | CSS `object-fit` |
| `opacity` | Opacity | `string` | `text` | `1` |  | CSS `opacity` |
| `border-radius` | Border Radius | `number` | `number` | `0` | `px` | CSS `border-radius` |
| `border` | Border | `number` | `number` | `0` | `px` | CSS border width |
| `border-style` | Border Style | `string` | `select` | `none` |  | CSS `border-style` |
| `background-color` | Background Color | `string` | `color` | `transparent` |  | CSS `background-color` |

## Select Options

### `text-align`

| Value | Label |
| --- | --- |
| `start` | Start |
| `left` | Left |
| `center` | Center |
| `right` | Right |
| `justify` | Justify |

### `font-family`

| Value | Label |
| --- | --- |
| `initial` | Initial |
| `Arial` | Arial |
| `Times New Roman` | Times New Roman |
| `Calibri` | Calibri |

### `font-weight`

| Value | Label |
| --- | --- |
| `normal` | Normal |
| `400` | Regular |
| `500` | Medium |
| `600` | Semi Bold |
| `700` | Bold |

### `font-style`

| Value | Label |
| --- | --- |
| `normal` | Normal |
| `italic` | Italic |

### `text-decoration`

| Value | Label |
| --- | --- |
| `none` | None |
| `underline` | Underline |
| `line-through` | Strike Through |

### `display`

| Value | Label |
| --- | --- |
| `inline` | Inline |
| `block` | Block |
| `grid` | Grid |
| `inline-block` | Inline Block |
| `none` | None |

### `align-items`

| Value | Label |
| --- | --- |
| `normal` | Normal |
| `stretch` | Stretch |
| `flex-start` | Flex Start |
| `center` | Center |
| `flex-end` | Flex End |
| `baseline` | Baseline |

### `justify-items`

| Value | Label | Native CSS |
| --- | --- | --- |
| `legacy` | Legacy | Yes |
| `stretch` | Stretch | Yes |
| `start` | Start | Yes |
| `center` | Center | Yes |
| `end` | End | Yes |

### `color`

| Value | Label |
| --- | --- |
| `canvastext` | Canvas Text |
| `#000000` | Black |
| `#2563EB` | Primary |
| `#64748B` | Secondary |
| `#7C3AED` | Accent |

### `background-color`

| Value | Label |
| --- | --- |
| `transparent` | Transparent |
| `#FFFFFF` | White |
| `#2563EB` | Primary |
| `#64748B` | Secondary |
| `#7C3AED` | Accent |

### `width`

| Value | Label |
| --- | --- |
| `auto` | Auto |
| `100%` | Full |
| `50%` | Half |
| `custom` | Custom |

### `height`

| Value | Label |
| --- | --- |
| `auto` | Auto |
| `100%` | Full |
| `custom` | Custom |

`custom` adalah mode UI untuk membuka input manual. Saat disimpan ke layer properties, frontend sebaiknya menyimpan value final CSS, contoh `75%`, `320px`, atau `fit-content`, bukan menyimpan `custom` sebagai ukuran final.

### `object-fit`

| Value | Label | Native CSS |
| --- | --- | --- |
| `fill` | Fill | Yes |
| `contain` | Contain | Yes |
| `cover` | Cover | Yes |

### `opacity`

| Value | Label |
| --- | --- |
| `1` | 100% |
| `0.5` | 50% |
| `0.25` | 25% |
| `0.1` | 10% |
| `custom` | Custom |

### `padding`

| Value | Label |
| --- | --- |
| `0` | 0px |
| `8` | 8px |
| `16` | 16px |
| `24` | 24px |

### `margin`

| Value | Label |
| --- | --- |
| `0` | 0px |
| `8` | 8px |
| `16` | 16px |
| `24` | 24px |

### `border`

| Value | Label |
| --- | --- |
| `0` | None |
| `1` | 1px |
| `2` | 2px |

### `border-style`

| Value | Label | Native CSS |
| --- | --- | --- |
| `none` | None | Yes |
| `solid` | Solid | Yes |
| `dashed` | Dashed | Yes |
| `dotted` | Dotted | Yes |
| `double` | Double | Yes |

### `border-radius`

| Value | Label |
| --- | --- |
| `0` | None |
| `4` | 4px |
| `8` | 8px |
| `16` | 16px |

### `font-size`

| Value | Label |
| --- | --- |
| `medium` | Medium |
| `12` | 12px |
| `14` | 14px |
| `16` | 16px |
| `20` | 20px |
| `24` | 24px |
| `custom` | Custom |

### `line-height`

| Value | Label |
| --- | --- |
| `normal` | Normal |
| `1` | Tight |
| `1.25` | Compact |
| `1.5` | Readable |
| `2` | Relaxed |

### `grid-template-columns`

| Value | Label | Meaning |
| --- | --- | --- |
| `none` | None | Native CSS initial value |
| `[100]` | Single | 1 column, 100% |
| `[50,50]` | Half | 2 columns, 50% / 50% |
| `[30,70]` | Left | 2 columns, 30% / 70% |
| `[70,30]` | Right | 2 columns, 70% / 30% |
| `[33,33,34]` | Thirds | 3 columns, total 100% |
| `custom` | Custom | UI mode for manual percentage columns |

`custom` adalah mode UI untuk membuka editor manual. Saat disimpan ke layer properties, frontend sebaiknya menyimpan value final JSON array persen, contoh `[25,25,50]`, bukan menyimpan `custom`.

## Element Property Mapping

### `grid`

| Position | Property Code | Default |
| --- | --- | --- |
| 1 | `display` | `grid` |
| 2 | `grid-template-columns` | `[100]` |
| 3 | `padding` | `0` |
| 4 | `margin` | `0` |
| 5 | `justify-items` | `stretch` |
| 6 | `align-items` | `stretch` |

### `text`

| Position | Property Code | Default |
| --- | --- | --- |
| 1 | `font-family` | `Arial` |
| 2 | `font-size` | `16` |
| 3 | `font-weight` | `400` |
| 4 | `font-style` | `normal` |
| 5 | `text-decoration` | `none` |
| 6 | `line-height` | `1.5` |
| 7 | `color` | `#000000` |
| 8 | `text-align` | `left` |
| 9 | `padding` | `0` |
| 10 | `margin` | `0` |

### `image`

| Position | Property Code | Default |
| --- | --- | --- |
| 1 | `width` | `auto` |
| 2 | `height` | `auto` |
| 3 | `object-fit` | `contain` |
| 4 | `border-radius` | `0` |
| 5 | `padding` | `0` |
| 6 | `margin` | `0` |

### `watermark`

| Position | Property Code | Default |
| --- | --- | --- |
| 1 | `width` | `50%` |
| 2 | `height` | `auto` |
| 3 | `object-fit` | `contain` |
| 4 | `opacity` | `0.1` |
| 5 | `padding` | `0` |
| 6 | `margin` | `0` |

### `table`

| Position | Property Code | Default |
| --- | --- | --- |
| 1 | `width` | `100%` |
| 2 | `font-size` | `14` |
| 3 | `color` | `#000000` |
| 4 | `border` | `1` |
| 5 | `border-style` | `solid` |
| 6 | `padding` | `0` |
| 7 | `margin` | `0` |

### `divider`

| Position | Property Code | Default |
| --- | --- | --- |
| 1 | `width` | `100%` |
| 2 | `height` | `1` |
| 3 | `background-color` | `#000000` |
| 4 | `padding` | `0` |
| 5 | `margin` | `0` |

### `spacer`

| Position | Property Code | Default |
| --- | --- | --- |
| 1 | `height` | `24` |

## Frontend Rendering Notes

### Unit Handling

Frontend dapat memakai helper sederhana:

```ts
function cssValue(value: string, unit?: string) {
  if (!unit) return value;
  if (value === "auto") return value;
  if (value.endsWith("%")) return value;
  return `${value}${unit}`;
}
```

### Grid Columns

`grid-template-columns` dikirim sebagai JSON string:

```json
"[50,50]"
```

Frontend dapat mengubahnya menjadi:

```css
grid-template-columns: 50% 50%;
```
