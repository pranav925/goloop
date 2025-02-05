/*
 * Copyright 2021 ICON Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package foundation.icon.ee.test;

import foundation.icon.ee.types.Address;
import foundation.icon.ee.types.Result;

import java.io.IOException;
import java.math.BigInteger;
import java.util.Map;

public interface InvokeHandler {
    static InvokeHandler defaultHandler() {
        return ServiceManager::sendInvokeAndWaitForResult;
    }

    Result invoke(
            ServiceManager sm,
            String code, boolean isReadOnly,
            Address from, Address to, BigInteger value,
            BigInteger stepLimit, String method, Object[] params,
            Map<String, Object> info, byte[] cid, int eid,
            Object[] codeState) throws IOException;
}
