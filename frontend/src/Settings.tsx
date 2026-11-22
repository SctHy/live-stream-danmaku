import {useState} from 'react'
import {createRoot} from 'react-dom/client'
import {settings, resetOverlay} from './danmaku'
import {Slider} from './ui/Slider'

const fields = [
  ['density', '同屏上限', 0, 160, '条'],
  ['area', '显示区域', 10, 100, '%'],
  ['opacity', '透明度', 10, 100, '%'],
  ['size', '字号', 14, 42, 'px'],
  ['speed', '速度', 60, 320, 'px/s']
] as const

function Settings() {
  const [current, setCurrent] = useState({...settings})
  function change<K extends keyof typeof settings>(key: K, value: typeof settings[K]) {
    settings[key] = value
    if (key !== 'opacity' && key !== 'speed') resetOverlay()
    setCurrent({...settings})
  }
  return <details className="settings"><summary>弹幕设置</summary><div className="fields">
    {fields.map(([key, label, min, max, unit]) => <div key={key}>
      <span>{label} <output>{current[key]} {unit}</output></span>
      <Slider label={label} value={current[key]} min={min} max={max} onChange={n => change(key,n)}/>
    </div>)}
    <label>文字颜色<input type="color" value={current.color} onChange={e => change('color', e.target.value)}/></label>
  </div></details>
}
createRoot(document.querySelector('#settings')!).render(<Settings/>)
