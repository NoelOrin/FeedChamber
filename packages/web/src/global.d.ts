import 'solid-js'

declare module 'solid-js' {
  namespace JSX {
    interface IntrinsicElements {
      'a-waves': {
        children?: any
        class?: string
        style?: string
      }
    }
  }
}

export {}
