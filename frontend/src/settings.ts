import {settings, resetOverlay} from './danmaku'

const fields = [
  ['density', '同屏上限', 0, 160, '条'],
  ['area', '显示区域', 10, 100, '%'],
  ['opacity', '透明度', 10, 100, '%'],
  ['size', '字号', 14, 42, 'px'],
  ['speed', '速度', 60, 320, 'px/s'],
] as const

const panel = document.querySelector('#settings .fields')!
for (const [key, name, min, max, unit] of fields) {
  const field = document.createElement('div')
  const label = document.createElement('label')
  const output = document.createElement('output')
  const input = document.createElement('input')
  input.type = 'range'
  input.id = 'setting-' + key
  label.htmlFor = input.id
  output.htmlFor.add(input.id)
  label.textContent = name
  input.min = String(min)
  input.max = String(max)
  input.value = String(settings[key])
  function render() {
    const value = input.valueAsNumber
    input.style.setProperty('--pct', (value - min) * 100 / (max - min) + '%')
    output.value = value + ' ' + unit
    input.setAttribute('aria-valuetext', output.value)
  }
  input.oninput = () => {
    settings[key] = input.valueAsNumber
    render()
    if (key !== 'speed' && key !== 'opacity') resetOverlay()
  }
  render()
  field.append(label, output, input)
  panel.append(field)
}

const colors = [['纯白', '#ffffff'], ['薄荷', '#34d399'], ['晴蓝', '#38bdf8'], ['琥珀', '#facc15'], ['珊瑚', '#fb7185']]
const palette = document.createElement('fieldset')
const legend = document.createElement('legend')
legend.textContent = '文字颜色'
palette.append(legend)
const picker = document.createElement('input')
picker.type = 'color'
picker.setAttribute('aria-label', '自定义文字颜色')
function selectColor(color: string) {
  settings.color = picker.value = color
  for (const button of Array.from(palette.querySelectorAll('button'))) {
    button.setAttribute('aria-pressed', String(button.value === color))
  }
  resetOverlay()
}
for (const [name, color] of colors) {
  const button = document.createElement('button')
  button.type = 'button'
  button.value = color
  button.title = name
  button.setAttribute('aria-label', name)
  button.setAttribute('aria-pressed', String(settings.color === color))
  button.style.setProperty('--swatch', color)
  button.onclick = () => selectColor(color)
  palette.append(button)
}
picker.value = settings.color
picker.oninput = () => selectColor(picker.value)
palette.append(picker)
panel.append(palette)
