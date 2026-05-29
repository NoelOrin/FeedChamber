const tagName = 'a-waves'
const definedTagNames = new Set<string>()

function getMousePosition(event: MouseEvent, element: HTMLElement): { x: number; y: number } {
  const rect = element.getBoundingClientRect()
  return {
    x: event.clientX - rect.left,
    y: event.clientY - rect.top,
  }
}

function createDots(svgElement: SVGSVGElement, count: number) {
  const ns = 'http://www.w3.org/2000/svg'
  const fragment = document.createDocumentFragment()

  for (let i = 0; i < count; i++) {
    const circle = document.createElementNS(ns, 'circle')
    circle.setAttribute('class', 'a__dot')
    circle.setAttribute('r', '0')
    circle.setAttribute('cx', '0')
    circle.setAttribute('cy', '0')
    fragment.appendChild(circle)
  }

  svgElement.appendChild(fragment)
  return svgElement.querySelectorAll('circle')
}

function updateDot(
  dot: SVGCircleElement,
  index: number,
  totalDots: number,
  width: number,
  height: number,
  mouseX: number,
  mouseY: number
) {
  const cols = Math.ceil(Math.sqrt(totalDots))
  const rows = Math.ceil(totalDots / cols)
  const col = index % cols
  const row = Math.floor(index / cols)

  const x = (width / (cols + 1)) * (col + 1)
  const y = (height / (rows + 1)) * (row + 1)

  const dx = mouseX - x
  const dy = mouseY - y
  const distance = Math.sqrt(dx * dx + dy * dy)

  const maxDistance = Math.sqrt(width * width + height * height) * 0.5
  const normalizedDistance = Math.min(distance / maxDistance, 1)
  const radius = (1 - normalizedDistance) * 4 + 1

  dot.setAttribute('cx', String(x))
  dot.setAttribute('cy', String(y))
  dot.setAttribute('r', String(radius))
  dot.style.opacity = String(1 - normalizedDistance * 0.7)
}

class AWavesElement extends HTMLElement {
  private svg: SVGSVGElement | null = null
  private dots: SVGCircleElement[] = []
  private mouseX = -1000
  private mouseY = -1000
  private animationId: number | null = null
  private boundMouseMove: ((e: MouseEvent) => void) | null = null
  private boundMouseLeave: (() => void) | null = null

  connectedCallback() {
    this.svg = this.querySelector('svg.js-svg') as SVGSVGElement | null
    if (!this.svg) return

    const rect = this.getBoundingClientRect()
    const width = rect.width
    const height = rect.height

    this.svg.setAttribute('viewBox', `0 0 ${width} ${height}`)
    this.svg.setAttribute('width', String(width))
    this.svg.setAttribute('height', String(height))

    const dotCount = Math.min(Math.floor((width * height) / 2500), 200)
    this.dots = Array.from(createDots(this.svg, dotCount))

    this.boundMouseMove = (e: MouseEvent) => {
      const pos = getMousePosition(e, this!)
      this.mouseX = pos.x
      this.mouseY = pos.y
      this!.style.setProperty('--x', `${pos.x}px`)
      this!.style.setProperty('--y', `${pos.y}px`)
    }

    this.boundMouseLeave = () => {
      this.mouseX = -1000
      this.mouseY = -1000
    }

    this.addEventListener('mousemove', this.boundMouseMove)
    this.addEventListener('mouseleave', this.boundMouseLeave)

    this.startAnimation()
  }

  disconnectedCallback() {
    if (this.animationId !== null) {
      cancelAnimationFrame(this.animationId)
      this.animationId = null
    }
    if (this.boundMouseMove) {
      this.removeEventListener('mousemove', this.boundMouseMove)
    }
    if (this.boundMouseLeave) {
      this.removeEventListener('mouseleave', this.boundMouseLeave)
    }
  }

  private startAnimation = () => {
    if (!this.svg) return

    const rect = this.getBoundingClientRect()
    const width = rect.width
    const height = rect.height
    const totalDots = this.dots.length

    for (let i = 0; i < totalDots; i++) {
      updateDot(this.dots[i], i, totalDots, width, height, this.mouseX, this.mouseY)
    }

    this.animationId = requestAnimationFrame(this.startAnimation)
  }
}

if (!definedTagNames.has(tagName)) {
  customElements.define(tagName, AWavesElement)
  definedTagNames.add(tagName)
}

export {}
