/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

import React, { useState, useEffect } from 'react'

import type { ItemType } from '@/components/miller-columns'
import { MillerColumns } from '@/components/miller-columns'

import {
  useJIRAMillerColumns,
  UseJIRAMillerColumnsProps
} from './use-jira-miller-columns'

interface Props extends UseJIRAMillerColumnsProps {
  onChangeItems: (items: Array<Pick<ItemType, 'id' | 'title'>>) => void
}

export const JIRAMillerColumns = ({ connectionId, onChangeItems }: Props) => {
  const [seletedIds, setSelectedIds] = useState<Array<ItemType['id']>>([])

  const { items, hasMore, onScroll } = useJIRAMillerColumns({ connectionId })

  useEffect(() => {
    onChangeItems(
      items
        .filter((it) => seletedIds.includes(it.id))
        .map((it) => ({
          id: it.id,
          title: it.title
        }))
    )
  }, [seletedIds])

  return (
    <MillerColumns
      height={300}
      columnCount={1}
      items={items}
      selectedItemIds={seletedIds}
      onSelectedItemIds={setSelectedIds}
      scrollProps={{
        hasMore,
        onScroll
      }}
    />
  )
}
