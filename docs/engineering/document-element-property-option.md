# Document Element Property Options

Dokumen ini merangkum flag property document builder yang disediakan dari seeder:

- `migration/document_properties.sql`
- `migration/document_property_options.sql`
- `migration/document_element_properties.sql`

Tujuannya agar frontend dapat melakukan sinkronisasi render, form editor, dan default value berdasarkan `code` property yang dikirim backend. Frontend sebaiknya menjadikan `code` sebagai key utama, sedangkan `token` hanya dipakai sebagai public identifier untuk komunikasi API.

## Sync Rules

- `property.code` adalah flag CSS/native rendering yang dipakai frontend, contoh `font-size`, `list-style-type`, `grid-template-columns`.
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

## Property Master

| Code | Name | Data Type | Input Type | Default | Unit | FE Usage |
| --- | --- | --- | --- | --- | --- | --- |
| `text-align` | Text Align | `string` | `select` | `left` |  | CSS `text-align` |
| `vertical-align` | Vertical Align | `string` | `select` | `top` |  | CSS `vertical-align` atau table/content alignment |
| `font-family` | Font Family | `string` | `select` | `Arial` |  | CSS `font-family` |
| `font-size` | Font Size | `number` | `number` | `16` | `px` | CSS `font-size` |
| `margin` | Margin | `number` | `number` | `24` | `px` | CSS `margin` |
| `padding` | Padding | `number` | `number` | `0` | `px` | CSS `padding` |
| `border` | Border | `number` | `number` | `1` | `px` | CSS border width |
| `border-style` | Border Style | `string` | `select` | `solid` |  | CSS `border-style` |
| `gap` | Gap | `number` | `number` | `0` | `px` | CSS `gap` |
| `font-weight` | Font Weight | `string` | `select` | `400` |  | CSS `font-weight` |
| `font-style` | Font Style | `string` | `select` | `normal` |  | CSS `font-style` |
| `text-decoration` | Text Decoration | `string` | `select` | `none` |  | CSS `text-decoration` |
| `line-height` | Line Height | `number` | `number` | `1.5` |  | CSS `line-height` |
| `color` | Color | `string` | `color` | `#000000` |  | CSS `color` |
| `width` | Width | `string` | `text` | `auto` |  | CSS `width` |
| `height` | Height | `string` | `text` | `auto` |  | CSS `height` |
| `object-fit` | Object Fit | `string` | `select` | `contain` |  | CSS `object-fit` |
| `display` | Display | `string` | `select` | `block` |  | CSS `display` |
| `grid-template-columns` | Grid Template Columns | `json` | `grid-columns` | `[100]` | `%` | CSS grid template columns from percentage array |
| `flex-direction` | Flex Direction | `string` | `select` | `row` |  | CSS `flex-direction` |
| `justify-content` | Justify Content | `string` | `select` | `flex-start` |  | CSS `justify-content` |
| `justify-items` | Justify Items | `string` | `select` | `stretch` |  | CSS `justify-items` |
| `align-items` | Align Items | `string` | `select` | `stretch` |  | CSS `align-items` |
| `list-style-type` | List Style Type | `string` | `select` | `disc` |  | CSS `list-style-type` |
| `list-style-position` | List Style Position | `string` | `select` | `inside` |  | CSS `list-style-position` |
| `border-radius` | Border Radius | `number` | `number` | `0` | `px` | CSS `border-radius` |
| `background-color` | Background Color | `string` | `color` | `#FFFFFF` |  | CSS `background-color` |
| `background-image` | Background Image | `string` | `text` |  |  | CSS `background-image` |

## Select Options

### `text-align`

| Value | Label |
| --- | --- |
| `left` | Left |
| `center` | Center |
| `right` | Right |
| `justify` | Justify |

### `vertical-align`

| Value | Label |
| --- | --- |
| `top` | Top |
| `middle` | Middle |
| `bottom` | Bottom |

### `font-family`

| Value | Label |
| --- | --- |
| `Arial` | Arial |
| `Times New Roman` | Times New Roman |
| `Calibri` | Calibri |

### `font-weight`

| Value | Label |
| --- | --- |
| `300` | Light |
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
| `block` | Block |
| `flex` | Flex |
| `grid` | Grid |
| `inline-block` | Inline Block |
| `none` | None |

### `flex-direction`

| Value | Label |
| --- | --- |
| `row` | Row |
| `column` | Column |
| `row-reverse` | Row Reverse |
| `column-reverse` | Column Reverse |

### `justify-content`

| Value | Label |
| --- | --- |
| `flex-start` | Flex Start |
| `center` | Center |
| `flex-end` | Flex End |
| `space-between` | Space Between |
| `space-around` | Space Around |
| `space-evenly` | Space Evenly |

### `align-items`

| Value | Label |
| --- | --- |
| `stretch` | Stretch |
| `flex-start` | Flex Start |
| `center` | Center |
| `flex-end` | Flex End |
| `baseline` | Baseline |

### `justify-items`

| Value | Label | Native CSS |
| --- | --- | --- |
| `stretch` | Stretch | Yes |
| `start` | Start | Yes |
| `center` | Center | Yes |
| `end` | End | Yes |

### `list-style-type`

| Value | Label | Native CSS |
| --- | --- | --- |
| `disc` | Disc | Yes |
| `circle` | Circle | Yes |
| `square` | Square | Yes |
| `decimal` | Decimal | Yes |
| `lower-alpha` | Lower Alpha | Yes |
| `upper-alpha` | Upper Alpha | Yes |
| `none` | None | Yes |

### `list-style-position`

| Value | Label | Native CSS |
| --- | --- | --- |
| `inside` | Inside | Yes |
| `outside` | Outside | Yes |

### `color`

| Value | Label |
| --- | --- |
| `#2563EB` | Primary |
| `#64748B` | Secondary |
| `#7C3AED` | Accent |

### `background-color`

| Value | Label |
| --- | --- |
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
| `contain` | Contain | Yes |
| `cover` | Cover | Yes |
| `fill` | Fill | Yes |

### `gap`

| Value | Label |
| --- | --- |
| `0` | 0px |
| `8` | 8px |
| `16` | 16px |
| `24` | 24px |

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
| `12` | 12px |
| `14` | 14px |
| `16` | 16px |
| `20` | 20px |
| `24` | 24px |

### `line-height`

| Value | Label |
| --- | --- |
| `1` | Tight |
| `1.25` | Compact |
| `1.5` | Normal |
| `2` | Relaxed |

### `grid-template-columns`

| Value | Label | Meaning |
| --- | --- | --- |
| `[100]` | Single | 1 column, 100% |
| `[50,50]` | Half | 2 columns, 50% / 50% |
| `[30,70]` | Left | 2 columns, 30% / 70% |
| `[70,30]` | Right | 2 columns, 70% / 30% |
| `[33,33,34]` | Thirds | 3 columns, total 100% |

## Element Property Mapping

### `grid`

| Position | Property Code | Default |
| --- | --- | --- |
| 1 | `display` | `grid` |
| 2 | `grid-template-columns` | `[100]` |
| 3 | `width` | `auto` |
| 4 | `height` | `auto` |
| 5 | `gap` | `0` |
| 6 | `padding` | `0` |
| 7 | `margin` | `0` |
| 8 | `background-color` | `#FFFFFF` |
| 9 | `border` | `0` |
| 10 | `border-style` | `solid` |
| 11 | `border-radius` | `0` |
| 12 | `justify-content` | `flex-start` |
| 13 | `justify-items` | `stretch` |
| 14 | `align-items` | `stretch` |

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
| 9 | `margin` | `0` |

### `image`

| Position | Property Code | Default |
| --- | --- | --- |
| 1 | `width` | `auto` |
| 2 | `height` | `auto` |
| 3 | `object-fit` | `contain` |
| 4 | `margin` | `0` |
| 5 | `border-radius` | `0` |

### `list`

| Position | Property Code | Default |
| --- | --- | --- |
| 1 | `list-style-type` | `disc` |
| 2 | `list-style-position` | `inside` |
| 3 | `font-family` | `Arial` |
| 4 | `font-size` | `16` |
| 5 | `font-weight` | `400` |
| 6 | `font-style` | `normal` |
| 7 | `text-decoration` | `none` |
| 8 | `line-height` | `1.5` |
| 9 | `color` | `#000000` |
| 10 | `margin` | `0` |
| 11 | `padding` | `0` |

### `table`

| Position | Property Code | Default |
| --- | --- | --- |
| 1 | `width` | `100%` |
| 2 | `font-size` | `14` |
| 3 | `color` | `#000000` |
| 4 | `border` | `1` |
| 5 | `border-style` | `solid` |
| 6 | `border-radius` | `0` |

### `divider`

| Position | Property Code | Default |
| --- | --- | --- |
| 1 | `width` | `100%` |
| 2 | `height` | `1` |
| 3 | `background-color` | `#000000` |
| 4 | `margin` | `0` |
| 5 | `padding` | `0` |

### `spacer`

| Position | Property Code | Default |
| --- | --- | --- |
| 1 | `height` | `24` |

### `signature`

| Position | Property Code | Default |
| --- | --- | --- |
| 1 | `width` | `240` |
| 2 | `height` | `96` |
| 3 | `margin` | `0` |
| 4 | `padding` | `0` |
| 5 | `border` | `0` |
| 6 | `border-style` | `solid` |
| 7 | `border-radius` | `0` |
| 8 | `text-align` | `center` |
| 9 | `font-family` | `Arial` |
| 10 | `font-size` | `14` |
| 11 | `font-weight` | `400` |
| 12 | `font-style` | `normal` |
| 13 | `text-decoration` | `none` |
| 14 | `color` | `#000000` |

### `qr-code`

| Position | Property Code | Default |
| --- | --- | --- |
| 1 | `width` | `120` |
| 2 | `height` | `120` |
| 3 | `margin` | `0` |
| 4 | `padding` | `0` |
| 5 | `background-color` | `#FFFFFF` |

### `barcode`

| Position | Property Code | Default |
| --- | --- | --- |
| 1 | `width` | `240` |
| 2 | `height` | `80` |
| 3 | `margin` | `0` |
| 4 | `background-color` | `#FFFFFF` |

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

### List Rendering

Untuk list, frontend sebaiknya melakukan reset native spacing agar hasil canvas konsisten:

```css
.document-layer ul,
.document-layer ol {
  margin: 0;
  padding: 0;
}
```

Lalu apply property:

```ts
const style = {
  listStyleType: properties["list-style-type"] ?? "disc",
  listStylePosition: properties["list-style-position"] ?? "inside",
  padding: cssValue(properties.padding ?? "0", "px"),
  margin: cssValue(properties.margin ?? "0", "px"),
};
```

Jika nantinya perlu mengatur indent list secara lebih presisi, property baru yang disarankan adalah `padding-left`, bukan `gap-list`, karena lebih native CSS dan mudah dipahami.
