#ifndef WAFFLE_OPERATION_H
#define WAFFLE_OPERATION_H

#include <string>
#include <vector>

class operation {
public:
    operation(const operation&) = default;
    operation& operator=(const operation&){
        return *this;
    };
    operation() : key(), value(), enqueueTime(){
    }

    ~operation() = default;
    std::string key;
    std::string value;
    int enqueueTime;

    bool operator == (const operation & rhs) const
    {
        if (!(key == rhs.key))
            return false;
        if (!(value == rhs.value))
            return false;
        return true;
    }
    bool operator != (const operation &rhs) const {
        return !(*this == rhs);
    }
};

#endif //WAFFLE_OPERATION_H