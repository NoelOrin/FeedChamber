import { formatSmartDate } from '../../utils/formatSmartDate'

interface BlogPropertiesProps {
  visits: number
  numComments: number
  createdAt: string
}

export default function BlogProperties(props: BlogPropertiesProps) {
  return (
    <div class="flex sm:flex-col md:flex-row w-full flex-wrap sm:justify-start md:justify-between properties sm:space-y-[4px] md:space-y-0 select-none">
      <div class="flex flex-1 space-x-[8px] sm:justify-between md:justify-start whitespace-nowrap">
        <div class="flex space-x-[8px]">
          <div class="flex items-center">
            <svg fill="currentColor" height="16" viewBox="0 0 24 24" width="16" xmlns="http://www.w3.org/2000/svg">
              <g>
                <path d="M12 3c5.392 0 9.878 3.88 10.819 9-.94 5.12-5.427 9-10.819 9-5.392 0-9.878-3.88-10.818-9C2.122 6.88 6.608 3 12 3Zm0 16a9.005 9.005 0 0 0 8.778-7 9.005 9.005 0 0 0-17.555 0A9.005 9.005 0 0 0 12 19Zm0-2.5a4.5 4.5 0 1 1 0-9 4.5 4.5 0 0 1 0 9Zm0-2a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5Z" />
              </g>
            </svg>
            <span class="mr-[4px]">浏览</span>
            <span>{props.visits}</span>
          </div>
          <div class="flex items-center">
            <svg fill="none" height="16" viewBox="0 0 48 48" width="16" xmlns="http://www.w3.org/2000/svg">
              <path d="M44 6H4V36H13V41L23 36H44V6Z" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="4" />
              <path d="M14 21H34" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="4" />
            </svg>
            <span class="mr-[4px]">评论</span>
            <span>{props.numComments}</span>
          </div>
        </div>
      </div>
      <div class="flex items-center">
        <span>
          <svg fill="none" height="16" viewBox="0 0 48 48" width="16" xmlns="http://www.w3.org/2000/svg">
            <path d="M24 44C35.0457 44 44 35.0457 44 24C44 12.9543 35.0457 4 24 4C12.9543 4 4 12.9543 4 24C4 35.0457 12.9543 44 24 44Z" fill="none" stroke="currentColor" stroke-linejoin="round" stroke-width="4" />
            <path d="M24.0084 12.0001L24.0072 24.0089L32.4866 32.4883" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="4" />
          </svg>
        </span>
        <span>{formatSmartDate(props.createdAt)}</span>
      </div>
    </div>
  )
}
