// Adapted from the shadcn/ui Slider (MIT), using the project theme.
import * as SliderPrimitive from '@radix-ui/react-slider'
export function Slider({label, value, min, max, onChange}: {
  label: string; value: number; min: number; max: number; onChange: (n: number) => void
}) {
  return <SliderPrimitive.Root className="slider" value={[value]} min={min} max={max} onValueChange={([n]) => onChange(n)}>
    <SliderPrimitive.Track className="slider-track"><SliderPrimitive.Range className="slider-range"/></SliderPrimitive.Track>
    <SliderPrimitive.Thumb className="slider-thumb" aria-label={label}/>
  </SliderPrimitive.Root>
}
