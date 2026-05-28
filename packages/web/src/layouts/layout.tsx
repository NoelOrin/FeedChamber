import type { JSX } from 'solid-js/jsx-runtime'
import Header from '~/layouts/common/header'
import Sidebar from '~/layouts/common/sidebar'
import Footer from '~/layouts/common/footer'

interface LayoutProps {
  children: JSX.Element
}

export default function Layout(props: LayoutProps) {
  return (
    <div>
      <Header />
      <div class="mx-auto flex-col justify-between py-1.5 min-h-[90vh] pt-[60px] sm:max-w-[1080px] xl:max-w-[1080px] 2xl:max-w-[1248px]">
        <div class="flex w-full h-full max-w-full">
          <Sidebar />
          <main class="mx-0 flex flex-1 flex-col p-2 min-w-0">
            {props.children}
          </main>
        </div>
      </div>
      <Footer />
    </div>
  )
}